package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#666666"))

	portStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Bold(true)

	portSystemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Bold(true)

	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C678DD"))

	pidStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#2C313A")).
				Bold(true)

	confirmStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E06C75")).
			MarginTop(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75")).
			MarginTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).
			MarginTop(1)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).
			Italic(true)
)

// --- Port scanning ---

type PortInfo struct {
	Port    int
	PID     int
	Command string
	User    string
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
			Port:    port,
			PID:     pid,
			Command: command,
			User:    user,
		})
	}

	return ports
}

// --- TUI Model ---

type state int

const (
	stateList state = iota
	stateConfirm
	stateResult
)

type model struct {
	ports      []PortInfo
	cursor     int
	state      state
	forceKill  bool
	message    string
	messageErr bool
	width      int
	height     int
}

func initialModel() model {
	return model{
		ports: getPorts(),
		state: stateList,
	}
}

type portsRefreshedMsg struct {
	ports []PortInfo
}

func refreshPorts() tea.Msg {
	return portsRefreshedMsg{ports: getPorts()}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case portsRefreshedMsg:
		m.ports = msg.ports
		if m.cursor >= len(m.ports) {
			m.cursor = max(0, len(m.ports)-1)
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "ctrl+c", "q":
		if m.state == stateConfirm {
			m.state = stateList
			m.forceKill = false
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		if m.state == stateConfirm || m.state == stateResult {
			m.state = stateList
			m.message = ""
			m.forceKill = false
			return m, nil
		}
		return m, tea.Quit
	}

	switch m.state {
	case stateList:
		return m.handleListKey(key)
	case stateConfirm:
		return m.handleConfirmKey(key)
	case stateResult:
		return m.handleResultKey(key)
	}

	return m, nil
}

func (m model) handleListKey(key string) (tea.Model, tea.Cmd) {
	if len(m.ports) == 0 {
		if key == "r" {
			return m, refreshPorts
		}
		return m, nil
	}

	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.ports)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.ports) - 1
	case "K":
		// Force kill — confirm with SIGKILL
		m.state = stateConfirm
		m.forceKill = true
	case "enter":
		// Kill — confirm with SIGTERM
		m.state = stateConfirm
		m.forceKill = false
	case "r":
		return m, refreshPorts
	}

	return m, nil
}

func (m model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "enter":
		p := m.ports[m.cursor]
		sig := syscall.SIGTERM
		sigName := "SIGTERM"
		if m.forceKill {
			sig = syscall.SIGKILL
			sigName = "SIGKILL"
		}

		err := syscall.Kill(p.PID, sig)
		if err != nil {
			m.message = fmt.Sprintf("Failed to kill PID %d: %s", p.PID, err)
			m.messageErr = true
		} else {
			m.message = fmt.Sprintf("Killed :%d (%s, PID %d) [%s]", p.Port, p.Command, p.PID, sigName)
			m.messageErr = false
		}
		m.state = stateResult
		m.forceKill = false
		return m, refreshPorts

	case "n":
		m.state = stateList
		m.forceKill = false
	}

	return m, nil
}

func (m model) handleResultKey(key string) (tea.Model, tea.Cmd) {
	m.state = stateList
	m.message = ""
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚡ portk"))
	b.WriteString("\n")

	if len(m.ports) == 0 {
		b.WriteString(emptyStyle.Render("  No listening ports found."))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  r refresh  q quit"))
		return b.String()
	}

	// Header
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-8s %-18s %-10s %s", "PORT", "COMMAND", "PID", "USER")))
	b.WriteString("\n")

	// Port list
	for i, p := range m.ports {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}

		pStyle := portStyle
		if p.Port < 1024 {
			pStyle = portSystemStyle
		}

		portStr := pStyle.Render(fmt.Sprintf(":%-7d", p.Port))
		cmdStr := commandStyle.Render(fmt.Sprintf("%-18s", truncate(p.Command, 18)))
		pidStr := pidStyle.Render(fmt.Sprintf("%-10d", p.PID))
		userStr := userStyle.Render(p.User)

		line := fmt.Sprintf("%s%s %s %s %s", cursor, portStr, cmdStr, pidStr, userStr)

		if i == m.cursor {
			line = selectedRowStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Confirm dialog
	if m.state == stateConfirm && m.cursor < len(m.ports) {
		p := m.ports[m.cursor]
		action := "Kill"
		sig := "SIGTERM"
		if m.forceKill {
			action = "Force kill"
			sig = "SIGKILL"
		}
		b.WriteString(confirmStyle.Render(
			fmt.Sprintf("  %s :%d (%s, PID %d) with %s? [y/n]", action, p.Port, p.Command, p.PID, sig),
		))
		b.WriteString("\n")
	}

	// Result message
	if m.state == stateResult && m.message != "" {
		style := successStyle
		prefix := "  ✓ "
		if m.messageErr {
			style = errorStyle
			prefix = "  ✗ "
		}
		b.WriteString(style.Render(prefix + m.message))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  Press any key to continue"))
		b.WriteString("\n")
	}

	// Help bar
	if m.state == stateList {
		b.WriteString(helpStyle.Render("  ↑/↓ navigate  enter kill  K force kill  r refresh  q quit"))
	}

	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// --- CLI fallback for non-interactive use ---

func main() {
	args := os.Args[1:]

	// If args given, use CLI mode
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printUsage()
		case "ls", "list":
			listPortsCLI()
		case "kill", "k":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: portk kill <port> [port...]\n")
				os.Exit(1)
			}
			for _, p := range args[1:] {
				killPortCLI(p, false)
			}
		case "kill!", "k!":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: portk kill! <port> [port...]\n")
				os.Exit(1)
			}
			for _, p := range args[1:] {
				killPortCLI(p, true)
			}
		default:
			if _, err := strconv.Atoi(args[0]); err == nil {
				for _, p := range args {
					killPortCLI(p, false)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
				printUsage()
				os.Exit(1)
			}
		}
		return
	}

	// Default: TUI mode
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`portk - fast port killer

USAGE
  portk                  Interactive TUI
  portk ls               List all listening ports
  portk kill <port>      Kill process on port (SIGTERM)
  portk kill! <port>     Force kill process on port (SIGKILL)
  portk <port>           Shorthand for kill

EXAMPLES
  portk 3000             Kill whatever runs on :3000
  portk kill 8080 3000   Kill :8080 and :3000
  portk kill! 5432       Force kill :5432`)
}

func listPortsCLI() {
	ports := getPorts()
	if len(ports) == 0 {
		fmt.Println("No listening ports found")
		return
	}
	fmt.Printf("%-8s %-18s %-10s %s\n", "PORT", "COMMAND", "PID", "USER")
	for _, p := range ports {
		fmt.Printf(":%-7d %-18s %-10d %s\n", p.Port, p.Command, p.PID, p.User)
	}
}

func killPortCLI(portStr string, force bool) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "'%s' is not a valid port\n", portStr)
		return
	}

	ports := getPorts()
	var info *PortInfo
	for _, p := range ports {
		if p.Port == port {
			info = &p
			break
		}
	}

	if info == nil {
		fmt.Printf(":%d — nothing listening\n", port)
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
		fmt.Fprintf(os.Stderr, "Failed to kill PID %d on :%d — %s\n", info.PID, port, err)
		return
	}

	fmt.Printf("✕ :%d — killed %s (PID %d) [%s]\n", port, info.Command, info.PID, sigName)
}
