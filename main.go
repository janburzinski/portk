package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B"))

	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	portStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Bold(true)

	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C678DD")).
			Bold(true)

	pidStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	rowStyle = lipgloss.NewStyle()

	selectedRowBg = lipgloss.NewStyle().
			Background(lipgloss.Color("#2C313A"))

	confirmStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E5C07B"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379"))

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).
			Italic(true)

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3E4451"))

	flagAllStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))
)

// --- Dev process detection ---

// Processes that are almost certainly dev servers
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
	"beam.smp":   true, // Elixir/Erlang
	"mix":        true,
	"iex":        true,
	"elixir":     true,
}

func isDevProcess(command string) bool {
	cmd := strings.ToLower(command)
	if devCommands[cmd] {
		return true
	}
	// Partial matches for truncated names
	for prefix := range devCommands {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

// --- Port scanning ---

type PortInfo struct {
	Port    int
	PID     int
	Command string
	User    string
}

func getPorts(showAll bool) []PortInfo {
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

		if !showAll && !isDevProcess(command) {
			continue
		}

		ports = append(ports, PortInfo{
			Port:    port,
			PID:     pid,
			Command: command,
			User:    user,
		})
	}

	// Sort by port number for readability
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	return ports
}

// --- TUI Model ---

type viewState int

const (
	stateList viewState = iota
	stateConfirm
)

type model struct {
	ports      []PortInfo
	cursor     int
	state      viewState
	forceKill  bool
	showAll    bool
	message    string
	messageErr bool
	width      int
	height     int
}

func initialModel() model {
	return model{
		ports: getPorts(false),
		state: stateList,
	}
}

type portsRefreshedMsg struct {
	ports []PortInfo
}

type clearMessageMsg struct{}

func (m model) refreshPorts() tea.Msg {
	return portsRefreshedMsg{ports: getPorts(m.showAll)}
}

func clearMessageAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearMessageMsg{}
	})
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

	case clearMessageMsg:
		m.message = ""

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if m.state == stateConfirm {
			m.state = stateList
			m.forceKill = false
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		if m.state == stateConfirm {
			m.state = stateList
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
	}

	return m, nil
}

func (m model) handleListKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if len(m.ports) > 0 && m.cursor < len(m.ports)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		if len(m.ports) > 0 {
			m.cursor = len(m.ports) - 1
		}
	case "enter":
		if len(m.ports) > 0 {
			m.state = stateConfirm
			m.forceKill = false
		}
	case "K":
		if len(m.ports) > 0 {
			m.state = stateConfirm
			m.forceKill = true
		}
	case "a":
		m.showAll = !m.showAll
		m.cursor = 0
		return m, m.refreshPorts
	case "r":
		return m, m.refreshPorts
	}

	return m, nil
}

func (m model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "enter":
		if m.cursor >= len(m.ports) {
			m.state = stateList
			return m, nil
		}
		p := m.ports[m.cursor]
		sig := syscall.SIGTERM
		sigName := "SIGTERM"
		if m.forceKill {
			sig = syscall.SIGKILL
			sigName = "SIGKILL"
		}

		err := syscall.Kill(p.PID, sig)
		if err != nil {
			m.message = fmt.Sprintf("✗ Failed to kill PID %d: %s", p.PID, err)
			m.messageErr = true
		} else {
			m.message = fmt.Sprintf("✓ Killed :%d  %s [%s]", p.Port, p.Command, sigName)
			m.messageErr = false
		}
		m.state = stateList
		m.forceKill = false
		return m, tea.Batch(m.refreshPorts, clearMessageAfter(3*time.Second))

	case "n", "esc":
		m.state = stateList
		m.forceKill = false
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Title + count
	title := titleStyle.Render("portk")
	filterLabel := "dev servers"
	if m.showAll {
		filterLabel = "all"
	}
	count := countStyle.Render(fmt.Sprintf(" %d ports (%s)", len(m.ports), filterLabel))
	b.WriteString(title + count + "\n")
	b.WriteString(separatorStyle.Render(strings.Repeat("─", 50)) + "\n")

	if len(m.ports) == 0 {
		b.WriteString("\n")
		b.WriteString(emptyStyle.Render("  No listening ports found."))
		b.WriteString("\n\n")
		help := "  a show all  r refresh  q quit"
		b.WriteString(helpStyle.Render(help))
		return b.String()
	}

	// Port list
	for i, p := range m.ports {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("› ")
		}

		portStr := portStyle.Render(fmt.Sprintf(":%-5d", p.Port))
		cmdStr := commandStyle.Render(fmt.Sprintf("%-15s", truncate(p.Command, 15)))
		pidStr := pidStyle.Render(fmt.Sprintf("PID %-7d", p.PID))

		line := fmt.Sprintf("%s%s  %s  %s", cursor, portStr, cmdStr, pidStr)

		if i == m.cursor {
			line = selectedRowBg.Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString(separatorStyle.Render(strings.Repeat("─", 50)) + "\n")

	// Status message (auto-clears after kill)
	if m.message != "" {
		style := successStyle
		if m.messageErr {
			style = errorMsgStyle
		}
		b.WriteString(style.Render("  "+m.message) + "\n")
	}

	// Confirm dialog
	if m.state == stateConfirm && m.cursor < len(m.ports) {
		p := m.ports[m.cursor]
		action := "Kill"
		if m.forceKill {
			action = "Force kill"
		}
		b.WriteString(confirmStyle.Render(
			fmt.Sprintf("  %s :%d (%s)? y/n", action, p.Port, p.Command),
		))
		b.WriteString("\n")
	}

	// Help
	if m.state == stateList {
		allToggle := "a all"
		if m.showAll {
			allToggle = "a dev only"
		}
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↑/↓ navigate  enter kill  K force  %s  r refresh  q quit", allToggle)))
	}

	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// --- CLI mode ---

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		showAll := false
		// Check for --all / -a flag
		filtered := make([]string, 0, len(args))
		for _, a := range args {
			if a == "--all" || a == "-a" {
				showAll = true
			} else {
				filtered = append(filtered, a)
			}
		}
		args = filtered

		if len(args) == 0 {
			// Just --all flag, launch TUI in all mode
			m := model{ports: getPorts(true), state: stateList, showAll: true}
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			return
		}

		switch args[0] {
		case "-h", "--help", "help":
			printUsage()
		case "ls", "list":
			listPortsCLI(showAll)
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

	// Default: TUI mode (dev servers only)
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
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

func listPortsCLI(showAll bool) {
	ports := getPorts(showAll)
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

	// Kill searches all ports, not just dev
	ports := getPorts(true)
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
