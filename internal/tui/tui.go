package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/janburzinski/portk/internal/ports"
)

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
)

type viewState int

const (
	stateList viewState = iota
	stateConfirm
)

type model struct {
	ports        []ports.Info
	cursor       int
	scrollOffset int
	state        viewState
	forceKill    bool
	showAll      bool
	message      string
	messageErr   bool
	width        int
	height       int
}

type portsRefreshedMsg struct {
	ports []ports.Info
}

type clearMessageMsg struct{}

func initialModel(showAll bool) model {
	return model{
		ports:   ports.List(showAll),
		state:   stateList,
		showAll: showAll,
	}
}

func (m model) refreshPorts() tea.Msg {
	return portsRefreshedMsg{ports: ports.List(m.showAll)}
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
		m.ensureCursorVisible()
	case portsRefreshedMsg:
		m.ports = msg.ports
		if m.cursor >= len(m.ports) {
			m.cursor = max(0, len(m.ports)-1)
		}
		m.ensureCursorVisible()
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
	case "q", "esc":
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
		m.ensureCursorVisible()
	case "down", "j":
		if len(m.ports) > 0 && m.cursor < len(m.ports)-1 {
			m.cursor++
		}
		m.ensureCursorVisible()
	case "pgup", "pageup", "ctrl+u":
		m.cursor = max(0, m.cursor-m.listHeight())
		m.ensureCursorVisible()
	case "pgdown", "pagedown", "ctrl+d":
		m.cursor = min(max(0, len(m.ports)-1), m.cursor+m.listHeight())
		m.ensureCursorVisible()
	case "home", "g":
		m.cursor = 0
		m.ensureCursorVisible()
	case "end", "G":
		if len(m.ports) > 0 {
			m.cursor = len(m.ports) - 1
		}
		m.ensureCursorVisible()
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
		m.scrollOffset = 0
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
		sigName, err := ports.KillPID(p.PID, m.forceKill)
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
		b.WriteString(helpStyle.Render("  a show all  r refresh  q quit"))
		return b.String()
	}

	visibleHeight := m.listHeight()
	start := min(m.scrollOffset, max(0, len(m.ports)-1))
	end := min(len(m.ports), start+visibleHeight)

	for i := start; i < end; i++ {
		p := m.ports[i]
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

	if m.message != "" {
		style := successStyle
		if m.messageErr {
			style = errorMsgStyle
		}
		b.WriteString(style.Render("  "+m.message) + "\n")
	}

	if m.state == stateConfirm && m.cursor < len(m.ports) {
		p := m.ports[m.cursor]
		action := "Kill"
		if m.forceKill {
			action = "Force kill"
		}
		b.WriteString(confirmStyle.Render(fmt.Sprintf("  %s :%d (%s)? y/n", action, p.Port, p.Command)))
		b.WriteString("\n")
	}

	if m.state == stateList {
		allToggle := "a all"
		if m.showAll {
			allToggle = "a dev only"
		}
		scrollHint := ""
		if len(m.ports) > visibleHeight {
			scrollHint = "  pgup/pgdn scroll"
		}
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↑/↓ navigate  enter kill  K force  %s  r refresh%s  q quit", allToggle, scrollHint)))
	}

	return b.String()
}

func (m *model) listHeight() int {
	if len(m.ports) == 0 {
		return 1
	}
	if m.height <= 0 {
		return len(m.ports)
	}

	// title + top separator + bottom separator + footer line
	height := m.height - 4
	if m.message != "" {
		height--
	}
	if height < 1 {
		height = 1
	}
	return height
}

func (m *model) ensureCursorVisible() {
	if len(m.ports) == 0 {
		m.cursor = 0
		m.scrollOffset = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.ports) {
		m.cursor = len(m.ports) - 1
	}

	visible := m.listHeight()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}

	maxOffset := max(0, len(m.ports)-visible)
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// Run starts the interactive TUI.
func Run(showAll bool) error {
	m := initialModel(showAll)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
