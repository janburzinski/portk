package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/janburzinski/portk/internal/ports"
)

// ExtractAllFlag parses --all / -a and returns cleaned args.
func ExtractAllFlag(args []string) (bool, []string) {
	showAll := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--all" || a == "-a" {
			showAll = true
			continue
		}
		filtered = append(filtered, a)
	}
	return showAll, filtered
}

// Execute handles CLI commands and returns an exit code.
func Execute(args []string, showAll bool) int {
	switch args[0] {
	case "-h", "--help", "help":
		PrintUsage()
		return 0
	case "ls", "list":
		ListPorts(showAll)
		return 0
	case "kill", "k":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: portk kill <port> [port...]")
			return 1
		}
		return KillPorts(args[1:], false)
	case "kill!", "k!":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: portk kill! <port> [port...]")
			return 1
		}
		return KillPorts(args[1:], true)
	default:
		if _, err := strconv.Atoi(args[0]); err == nil {
			return KillPorts(args, false)
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		PrintUsage()
		return 1
	}
}

func PrintUsage() {
	fmt.Println(`portk - fast port killer

USAGE
  portk                  Interactive TUI (dev servers only)
  portk -a               Interactive TUI (all ports)
  portk ls               List dev server ports
  portk ls -a            List all listening ports
  portk kill <port>      Kill process on port (SIGTERM)
  portk kill! <port>     Force kill (SIGKILL)
  portk <port>           Shorthand for kill

TUI KEYS
  ↑/↓ j/k   Navigate
  enter      Kill (SIGTERM) with confirmation
  K          Force kill (SIGKILL) with confirmation
  a          Toggle dev servers / all ports
  r          Refresh
  q          Quit`)
}

func ListPorts(showAll bool) {
	list := ports.List(showAll)
	if len(list) == 0 {
		fmt.Println("No listening ports found")
		return
	}
	fmt.Printf("%-8s %-18s %-10s %s\n", "PORT", "COMMAND", "PID", "USER")
	for _, p := range list {
		fmt.Printf(":%-7d %-18s %-10d %s\n", p.Port, p.Command, p.PID, p.User)
	}
}

func KillPorts(portArgs []string, force bool) int {
	exitCode := 0
	for _, portArg := range portArgs {
		if err := killOne(portArg, force); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

func killOne(portStr string, force bool) error {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "'%s' is not a valid port\n", portStr)
		return err
	}

	info := ports.FindByPort(port)
	if info == nil {
		fmt.Printf(":%d — nothing listening\n", port)
		return nil
	}

	sigName, err := ports.KillPID(info.PID, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to kill PID %d on :%d — %s\n", info.PID, port, err)
		return err
	}

	fmt.Printf("✕ :%d — killed %s (PID %d) [%s]\n", port, info.Command, info.PID, sigName)
	return nil
}
