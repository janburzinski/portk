package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	cyan    = "\033[36m"
	bgRed   = "\033[41m"
	white   = "\033[97m"
)

type PortInfo struct {
	Proto   string
	Port    int
	PID     int
	Command string
	User    string
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		interactive()
		return
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
	case "ls", "list":
		listPorts()
	case "kill", "k":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "%sUsage: portk kill <port> [port...]%s\n", red, reset)
			os.Exit(1)
		}
		for _, p := range args[1:] {
			killPort(p, false)
		}
	case "kill!", "k!":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "%sUsage: portk kill! <port> [port...]%s\n", red, reset)
			os.Exit(1)
		}
		for _, p := range args[1:] {
			killPort(p, true)
		}
	default:
		// If arg is a number, treat as kill
		if _, err := strconv.Atoi(args[0]); err == nil {
			for _, p := range args {
				killPort(p, false)
			}
		} else {
			fmt.Fprintf(os.Stderr, "%sUnknown command: %s%s\n", red, args[0], reset)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Printf(`%sportk%s - fast port killer

%sUSAGE%s
  portk                  Interactive mode
  portk ls               List all listening ports
  portk kill <port>      Kill process on port (SIGTERM)
  portk kill! <port>     Force kill process on port (SIGKILL)
  portk <port>           Shorthand for kill
  portk <port> <port>    Kill multiple ports

%sEXAMPLES%s
  portk 3000             Kill whatever runs on :3000
  portk kill 8080 3000   Kill :8080 and :3000
  portk kill! 5432       Force kill :5432
`, bold, reset, dim, reset, dim, reset)
}

func getPorts() []PortInfo {
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-nP")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var ports []PortInfo
	seen := make(map[int]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Scan() // skip header

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 9 {
			continue
		}

		command := fields[0]
		pid, _ := strconv.Atoi(fields[1])
		user := fields[2]
		name := fields[8]

		// Parse port from "host:port" or "*:port"
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

		ports = append(ports, PortInfo{
			Proto:   "TCP",
			Port:    port,
			PID:     pid,
			Command: command,
			User:    user,
		})
	}

	return ports
}

func listPorts() {
	ports := getPorts()
	if len(ports) == 0 {
		fmt.Printf("%sNo listening ports found%s\n", dim, reset)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s%sPORT\tPID\tCOMMAND\tUSER%s\n", bold, dim, reset)
	for _, p := range ports {
		portColor := cyan
		if p.Port < 1024 {
			portColor = yellow
		}
		fmt.Fprintf(w, "%s:%d%s\t%d\t%s%s%s\t%s\n",
			portColor, p.Port, reset,
			p.PID,
			bold, p.Command, reset,
			p.User,
		)
	}
	w.Flush()
}

func findByPort(port int) *PortInfo {
	ports := getPorts()
	for _, p := range ports {
		if p.Port == port {
			return &p
		}
	}
	return nil
}

func killPort(portStr string, force bool) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s'%s' is not a valid port%s\n", red, portStr, reset)
		return
	}

	info := findByPort(port)
	if info == nil {
		fmt.Printf("%s:%d%s — %snothing listening%s\n", dim, port, reset, dim, reset)
		return
	}

	sig := syscall.SIGTERM
	sigName := "SIGTERM"
	if force {
		sig = syscall.SIGKILL
		sigName = "SIGKILL"
	}

	err = syscall.Kill(info.PID, sig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sFailed to kill PID %d on :%d — %s%s\n", red, info.PID, port, err, reset)
		return
	}

	fmt.Printf("%s%s ✕ :%d%s — killed %s%s%s (PID %d) [%s]\n",
		bgRed, white, port, reset,
		bold, info.Command, reset,
		info.PID, sigName,
	)
}

func interactive() {
	ports := getPorts()
	if len(ports) == 0 {
		fmt.Printf("%sNo listening ports found%s\n", dim, reset)
		return
	}

	fmt.Printf("%s%sportk%s — interactive mode (q to quit)\n\n", bold, cyan, reset)
	printNumberedPorts(ports)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\n%s❯%s ", green, reset)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}
		if input == "q" || input == "quit" || input == "exit" {
			return
		}
		if input == "r" || input == "refresh" {
			ports = getPorts()
			if len(ports) == 0 {
				fmt.Printf("%sNo listening ports found%s\n", dim, reset)
				return
			}
			printNumberedPorts(ports)
			continue
		}
		if input == "?" || input == "help" {
			fmt.Printf("  %s#%s      Kill by list number\n", dim, reset)
			fmt.Printf("  %s:port%s  Kill by port number\n", dim, reset)
			fmt.Printf("  %sr%s     Refresh list\n", dim, reset)
			fmt.Printf("  %sq%s     Quit\n", dim, reset)
			continue
		}

		// Handle "all" — kill everything
		if input == "all" {
			for _, p := range ports {
				killByInfo(p, false)
			}
			ports = getPorts()
			if len(ports) == 0 {
				fmt.Printf("\n%sAll ports cleared%s\n", green, reset)
				return
			}
			printNumberedPorts(ports)
			continue
		}

		// Check if input starts with "!" for force kill
		force := false
		cleanInput := input
		if strings.HasSuffix(input, "!") {
			force = true
			cleanInput = strings.TrimSuffix(input, "!")
		}

		// Try as list index
		if idx, err := strconv.Atoi(cleanInput); err == nil {
			if idx >= 1 && idx <= len(ports) {
				killByInfo(ports[idx-1], force)
				ports = getPorts()
				if len(ports) == 0 {
					fmt.Printf("\n%sAll ports cleared%s\n", green, reset)
					return
				}
				printNumberedPorts(ports)
				continue
			}
			// Treat as port number
			killPort(cleanInput, force)
			ports = getPorts()
			if len(ports) == 0 {
				fmt.Printf("\n%sAll ports cleared%s\n", green, reset)
				return
			}
			printNumberedPorts(ports)
			continue
		}

		fmt.Printf("%sUnknown input. Type ? for help%s\n", dim, reset)
	}
}

func printNumberedPorts(ports []PortInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for i, p := range ports {
		portColor := cyan
		if p.Port < 1024 {
			portColor = yellow
		}
		fmt.Fprintf(w, "  %s%d)%s\t%s:%d%s\t%s%s%s\t%sPID %d%s\t%s\n",
			dim, i+1, reset,
			portColor, p.Port, reset,
			bold, p.Command, reset,
			dim, p.PID, reset,
			p.User,
		)
	}
	w.Flush()
}

func killByInfo(info PortInfo, force bool) {
	sig := syscall.SIGTERM
	sigName := "SIGTERM"
	if force {
		sig = syscall.SIGKILL
		sigName = "SIGKILL"
	}

	err := syscall.Kill(info.PID, sig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sFailed to kill PID %d on :%d — %s%s\n", red, info.PID, info.Port, err, reset)
		return
	}

	fmt.Printf("  %s%s ✕ :%d%s — killed %s%s%s (PID %d) [%s]\n",
		bgRed, white, info.Port, reset,
		bold, info.Command, reset,
		info.PID, sigName,
	)
}
