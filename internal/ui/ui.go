package ui

import (
	"fmt"
	"strings"
	"time"

	"ssh-deploy-tui/internal/config"
	"ssh-deploy-tui/internal/executor"
	"ssh-deploy-tui/internal/ssh"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewSplash viewState = iota
	viewConnecting
	viewMainMenu
	viewSelectProject
	viewDeploying
	viewResult
)

const AppVersion = "1.0.0"

type menuItem struct {
	title       string
	description string
}

var mainMenuItems = []menuItem{
	{title: "Deploy proyecto", description: "Pull, install, build y restart"},
	{title: "Salir", description: "Cerrar aplicacion"},
}

type Model struct {
	config          *config.Config
	sshClient       *ssh.Client
	state           viewState
	cursor          int
	selectedProject string
	spinner         spinner.Model
	result          string
	resultSuccess   bool
	projectKeys     []string
	width           int
	height          int
	connectionError string
	splashTick      int
}

// Mensajes
type sshConnectedMsg struct{ err error }
type deployDoneMsg struct {
	success bool
	message string
}
type splashTickMsg struct{}

func NewModel(cfg *config.Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		config:      cfg,
		sshClient:   ssh.NewClient(&cfg.SSH),
		state:       viewSplash,
		cursor:      0,
		spinner:     s,
		projectKeys: cfg.GetProjectList(),
		splashTick:  0,
	}
}

func splashTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return splashTickMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, splashTick())
}

func (m Model) connectSSH() tea.Cmd {
	return func() tea.Msg {
		err := m.sshClient.Connect()
		return sshConnectedMsg{err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == viewSplash {
			m.state = viewConnecting
			return m, m.connectSSH()
		}
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case splashTickMsg:
		if m.state == viewSplash {
			m.splashTick++
			return m, splashTick()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sshConnectedMsg:
		if msg.err != nil {
			m.connectionError = msg.err.Error()
			m.state = viewResult
			m.result = fmt.Sprintf("Error de conexion SSH:\n\n%s\n\nVerifica tu config.yaml", msg.err.Error())
			m.resultSuccess = false
		} else {
			m.state = viewMainMenu
		}
		return m, nil

	case deployDoneMsg:
		m.state = viewResult
		m.result = msg.message
		m.resultSuccess = msg.success
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.state == viewMainMenu {
			m.sshClient.Close()
			return m, tea.Quit
		}
		if m.connectionError != "" {
			return m, tea.Quit
		}
		m.state = viewMainMenu
		m.cursor = 0
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		m.cursor = m.incrementCursor()
		return m, nil

	case "enter", " ":
		return m.handleSelect()

	case "esc":
		if m.connectionError != "" {
			return m, tea.Quit
		}
		if m.state != viewMainMenu {
			m.state = viewMainMenu
			m.cursor = 0
		}
		return m, nil
	}

	return m, nil
}

func (m Model) incrementCursor() int {
	max := 0
	switch m.state {
	case viewMainMenu:
		max = len(mainMenuItems) - 1
	case viewSelectProject:
		max = len(m.projectKeys) - 1
	}
	if m.cursor < max {
		return m.cursor + 1
	}
	return m.cursor
}

func (m Model) handleSelect() (tea.Model, tea.Cmd) {
	switch m.state {
	case viewMainMenu:
		return m.handleMainMenuSelect()
	case viewSelectProject:
		return m.handleProjectSelect()
	case viewResult:
		if m.connectionError != "" {
			return m, tea.Quit
		}
		m.state = viewMainMenu
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m Model) handleMainMenuSelect() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0: // Deploy
		m.state = viewSelectProject
		m.cursor = 0
	case 1: // Salir
		m.sshClient.Close()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleProjectSelect() (tea.Model, tea.Cmd) {
	m.selectedProject = m.projectKeys[m.cursor]
	project, _ := m.config.GetProject(m.selectedProject)

	m.state = viewDeploying
	return m, m.doDeploy(project)
}

func (m Model) doDeploy(project config.Project) tea.Cmd {
	return func() tea.Msg {
		exec := executor.New(project, m.sshClient)
		outputChan := make(chan string, 10)

		go func() {
			for range outputChan {
			}
		}()

		err := exec.Deploy(outputChan)
		close(outputChan)

		if err != nil {
			return deployDoneMsg{success: false, message: fmt.Sprintf("Error: %v", err)}
		}

		results := exec.GetResults()
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("deploy de %s\n\n", project.Name))
		for _, r := range results {
			if r.Success {
				sb.WriteString(fmt.Sprintf("  %s %s\n", IconCheck, r.Step))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s: %s\n", IconCross, r.Step, r.Error))
			}
		}
		return deployDoneMsg{success: true, message: sb.String()}
	}
}

// ── Views ──

func (m Model) View() string {
	if m.state == viewSplash {
		return m.renderSplash()
	}

	var s strings.Builder

	s.WriteString(m.renderHeader())
	s.WriteString("\n\n")

	switch m.state {
	case viewConnecting:
		s.WriteString(m.renderConnecting())
	case viewMainMenu:
		s.WriteString(m.renderMainMenu())
	case viewSelectProject:
		s.WriteString(m.renderProjectSelect())
	case viewDeploying:
		s.WriteString(m.renderDeploying())
	case viewResult:
		s.WriteString(m.renderResult())
	}

	help := helpStyle.Render("up/down: navegar | enter: seleccionar | esc/q: volver | ctrl+c: salir")
	s.WriteString("\n")
	s.WriteString(help)

	return s.String()
}

func (m Model) renderSplash() string {
	var s strings.Builder

	selectedLogo := LogoClean

	logoLineCount := len(strings.Split(selectedLogo, "\n"))
	totalHeight := logoLineCount + 8
	topPadding := (m.height - totalHeight) / 2
	if topPadding < 0 {
		topPadding = 1
	}

	for i := 0; i < topPadding; i++ {
		s.WriteString("\n")
	}

	logoLines := strings.Split(selectedLogo, "\n")
	totalLogoLines := len(logoLines)
	for i, line := range logoLines {
		if line == "" {
			s.WriteString("\n")
			continue
		}
		var style lipgloss.Style
		linePos := float64(i) / float64(totalLogoLines)
		switch {
		case linePos < 0.33:
			style = logoGradientStyle1
		case linePos < 0.66:
			style = logoGradientStyle2
		default:
			style = logoGradientStyle3
		}
		styledLine := style.Render(line)
		centered := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styledLine)
		s.WriteString(centered)
		s.WriteString("\n")
	}

	decorWidth := 35
	if m.width < 50 {
		decorWidth = 20
	}
	decorLine := strings.Repeat("─", decorWidth)
	s.WriteString("\n")
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, accentLineStyle.Render(decorLine)))
	s.WriteString("\n\n")

	subtitle := "Deploy System"
	if m.width >= 50 {
		subtitle = "Cloud Remote Server | Deploy System"
	}
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, splashSubtitleStyle.Render(subtitle)))
	s.WriteString("\n\n")

	version := fmt.Sprintf(" v%s ", AppVersion)
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, versionBadgeStyle.Render(version)))
	s.WriteString("\n\n")

	blinkChars := []string{"▸", "▹"}
	blinkChar := blinkChars[(m.splashTick/3)%len(blinkChars)]
	pressKeyText := fmt.Sprintf("%s press any key to continue %s", blinkChar, blinkChar)
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, mutedStyle.Render(pressKeyText)))

	return s.String()
}

func (m Model) renderHeader() string {
	var s strings.Builder

	appName := m.config.AppName
	if appName == "" {
		appName = "SSH Deploy"
	}
	titleText := fmt.Sprintf("%s %s", IconTerminal, appName)
	title := titleStyle.Render(titleText)

	var status string
	if m.sshClient.IsConnected() {
		status = statusOnlineStyle.Render(fmt.Sprintf("%s %s", IconFilled, m.sshClient.GetHost()))
	} else if m.state == viewConnecting {
		status = mutedStyle.Render(fmt.Sprintf("%s connecting...", IconCircle))
	} else {
		status = statusOfflineStyle.Render(fmt.Sprintf("%s offline", IconCircle))
	}

	s.WriteString(title)
	s.WriteString("  ")
	s.WriteString(status)
	s.WriteString("\n")

	separator := strings.Repeat("─", 40)
	s.WriteString(accentLineStyle.Render(separator))

	return s.String()
}

func (m Model) renderConnecting() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n  %s Establishing SSH connection...\n", m.spinner.View()))
	s.WriteString(fmt.Sprintf("     %s %s\n", IconSSH, mutedStyle.Render(m.config.SSH.Host)))
	return s.String()
}

var menuIcons = []string{"◈", "◇"}

func (m Model) renderMainMenu() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("  select operation"))
	s.WriteString("\n\n")

	for i, item := range mainMenuItems {
		icon := menuIcons[i%len(menuIcons)]
		if i == m.cursor {
			s.WriteString(fmt.Sprintf("  %s %s %s\n",
				selectedStyle.Render(IconArrow),
				selectedStyle.Render(icon),
				selectedStyle.Render(item.title)))
			s.WriteString(fmt.Sprintf("      %s\n", subtitleStyle.Render(item.description)))
		} else {
			s.WriteString(fmt.Sprintf("    %s %s\n",
				mutedStyle.Render(icon),
				normalStyle.Render(item.title)))
		}
	}

	return s.String()
}

func (m Model) renderProjectSelect() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("  select project"))
	s.WriteString("\n\n")

	for i, key := range m.projectKeys {
		project, _ := m.config.GetProject(key)

		typeIcon := IconFolder
		if project.Type == "pm2" {
			typeIcon = IconServer
		}

		if i == m.cursor {
			line := fmt.Sprintf("%s %s", project.Name, mutedStyle.Render(fmt.Sprintf("(%s)", project.Branch)))
			s.WriteString(fmt.Sprintf("  %s %s %s\n",
				selectedStyle.Render(IconArrow),
				selectedStyle.Render(typeIcon),
				selectedStyle.Render(line)))
		} else {
			line := fmt.Sprintf("%s (%s)", project.Name, project.Branch)
			s.WriteString(fmt.Sprintf("    %s %s\n",
				mutedStyle.Render(typeIcon),
				normalStyle.Render(line)))
		}
	}

	return s.String()
}

func (m Model) renderDeploying() string {
	return fmt.Sprintf("%s ejecutando deploy...\n", m.spinner.View())
}

func (m Model) renderResult() string {
	var s strings.Builder
	s.WriteString("\n")

	if m.resultSuccess {
		s.WriteString(successStyle.Render(fmt.Sprintf("  %s operation completed", IconCheck)))
	} else {
		s.WriteString(errorStyle.Render(fmt.Sprintf("  %s operation failed", IconCross)))
	}
	s.WriteString("\n\n")

	maxLines := m.height - 10
	if maxLines < 10 {
		maxLines = 10
	}

	lines := strings.Split(m.result, "\n")

	if len(lines) > maxLines {
		s.WriteString(mutedStyle.Render(fmt.Sprintf("  showing last %d of %d lines\n\n", maxLines, len(lines))))
		lines = lines[len(lines)-maxLines:]
	}
	s.WriteString(strings.Join(lines, "\n"))

	s.WriteString("\n\n")
	s.WriteString(subtitleStyle.Render("enter o esc para volver"))

	return s.String()
}
