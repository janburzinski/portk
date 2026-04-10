package ports

import (
	"bufio"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// Info holds process details for a listening TCP port.
type Info struct {
	Port    int
	PID     int
	Command string
	User    string
}

// Commands that are highly likely to be local dev/runtime processes.
var devCommands = map[string]bool{
	"node":       true,
	"bun":        true,
	"deno":       true,
	"tsx":        true,
	"ts-node":    true,
	"npx":        true,
	"next-serve": true,
	"vite":       true,
	"esbuild":    true,
	"python":     true,
	"python3":    true,
	"uvicorn":    true,
	"gunicorn":   true,
	"flask":      true,
	"django":     true,
	"ruby":       true,
	"rails":      true,
	"puma":       true,
	"go":         true,
	"air":        true,
	"cargo":      true,
	"java":       true,
	"gradle":     true,
	"mvn":        true,
	"php":        true,
	"artisan":    true,
	"nginx":      true,
	"caddy":      true,
	"hugo":       true,
	"jekyll":     true,
	"webpack":    true,
	"parcel":     true,
	"turbo":      true,
	"wrangler":   true,
	"miniflare":  true,
	"workerd":    true,
	"handler":    true,
	"doppler":    true,
	"docker-pro": true,
	"com.docke":  true,
	"redis-ser":  true,
	"postgres":   true,
	"mysqld":     true,
	"mongod":     true,
	"beam.smp":   true,
	"mix":        true,
	"iex":        true,
	"elixir":     true,
}

func isDevProcess(command string) bool {
	cmd := strings.ToLower(command)
	if devCommands[cmd] {
		return true
	}
	for prefix := range devCommands {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

// List returns current listening TCP ports from lsof.
func List(showAll bool) []Info {
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-nP")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseLsofOutput(string(out), showAll)
}

func parseLsofOutput(output string, showAll bool) []Info {
	var result []Info
	seen := make(map[int]bool)
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Scan() // header

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 9 {
			continue
		}

		command := fields[0]
		pid, _ := strconv.Atoi(fields[1])
		user := fields[2]
		name := fields[8]

		idx := strings.LastIndex(name, ":")
		if idx == -1 {
			continue
		}

		port, err := strconv.Atoi(name[idx+1:])
		if err != nil {
			continue
		}

		if seen[port] {
			continue
		}
		seen[port] = true

		if !showAll && !isDevProcess(command) {
			continue
		}

		result = append(result, Info{
			Port:    port,
			PID:     pid,
			Command: command,
			User:    user,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Port < result[j].Port
	})

	return result
}

// FindByPort returns the listening process on a port (searches all processes).
func FindByPort(port int) *Info {
	for _, p := range List(true) {
		if p.Port == port {
			found := p
			return &found
		}
	}
	return nil
}

// KillPID sends SIGTERM or SIGKILL.
func KillPID(pid int, force bool) (string, error) {
	sig := syscall.SIGTERM
	sigName := "SIGTERM"
	if force {
		sig = syscall.SIGKILL
		sigName = "SIGKILL"
	}

	if err := syscall.Kill(pid, sig); err != nil {
		return sigName, err
	}

	return sigName, nil
}
