package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ssh-deploy-tui/internal/config"
	"ssh-deploy-tui/internal/executor"
	"ssh-deploy-tui/internal/ssh"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewSplash viewState = iota
	viewConnecting
	viewMainMenu
	viewSelectProject
	viewSelectLogType
	viewDeploying
	viewLogs
	viewLogsStream
	viewStatus
	viewNginx
	viewResult
	viewScrollable
)

const AppVersion = "1.0.0"

type menuItem struct {
	title       string
	description string
}

var mainMenuItems = []menuItem{
	{title: "Deploy proyecto", description: "Pull, install, build y restart"},
	{title: "Ver logs", description: "Mostrar logs de PM2"},
	{title: "Restart servicio", description: "Reiniciar sin rebuild"},
	{title: "Status PM2", description: "Ver estado de procesos"},
	{title: "Nginx", description: "Reload o test config"},
	{title: "Salir", description: "Cerrar aplicacion"},
}

var logTypeMenuItems = []menuItem{
	{title: "Tiempo real", description: "Streaming en vivo (auto-refresh)"},
	{title: "Ultimas 100 lineas", description: "Ver logs historicos"},
	{title: "← Volver", description: "Menu principal"},
}

var nginxMenuItems = []menuItem{
	{title: "Ver configuracion", description: "Mostrar sites-available"},
	{title: "Copiar config", description: "Copiar al clipboard"},
	{title: "Test config", description: "Verificar configuracion"},
	{title: "Reload", description: "Recargar nginx"},
	{title: "← Volver", description: "Menu principal"},
}

// Buffer compartido para logs en streaming
type logBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (b *logBuffer) append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
	if len(b.lines) > 200 {
		b.lines = b.lines[1:]
	}
}

func (b *logBuffer) getAll() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

type Model struct {
	config          *config.Config
	sshClient       *ssh.Client
	state           viewState
	cursor          int
	selectedProject string
	selectedAction  string
	spinner         spinner.Model
	logs            []string
	result          string
	resultSuccess   bool
	projectKeys     []string
	width           int
	height          int
	connectionError string
	// Streaming
	streamStopCh  chan struct{}
	streaming     bool
	logBuffer     *logBuffer
	streamPm2Name string
	// Viewport para scroll
	viewport      viewport.Model
	viewportReady bool
	viewportTitle string
	// Splash screen
	splashTick int
}

// Mensajes
type sshConnectedMsg struct{ err error }
type deployDoneMsg struct {
	success bool
	message string
}
type logsMsg struct {
	logs []string
	err  error
}
type streamTickMsg struct{}
type statusMsg struct {
	status string
	err    error
}
type nginxMsg struct {
	output  string
	success bool
}
type scrollableContentMsg struct {
	title   string
	content string
	err     error
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
		logBuffer:   &logBuffer{lines: []string{}},
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

func streamTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return streamTickMsg{}
	})
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
		if m.viewportReady {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}
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

	case logsMsg:
		m.state = viewLogs
		if msg.err != nil {
			m.logs = []string{fmt.Sprintf("Error: %v", msg.err)}
		} else {
			m.logs = msg.logs
		}
		return m, nil

	case streamTickMsg:
		if m.streaming && m.state == viewLogsStream {
			m.logs = m.logBuffer.getAll()
			return m, streamTick()
		}
		return m, nil

	case statusMsg:
		m.state = viewResult
		if msg.err != nil {
			m.result = fmt.Sprintf("Error: %v", msg.err)
			m.resultSuccess = false
		} else {
			m.result = msg.status
			m.resultSuccess = true
		}
		return m, nil

	case nginxMsg:
		m.state = viewResult
		m.result = msg.output
		m.resultSuccess = msg.success
		return m, nil

	case scrollableContentMsg:
		if msg.err != nil {
			m.state = viewResult
			m.result = fmt.Sprintf("Error: %v", msg.err)
			m.resultSuccess = false
			return m, nil
		}
		m.viewport = viewport.New(m.width, m.height-6)
		m.viewport.SetContent(msg.content)
		m.viewportReady = true
		m.viewportTitle = msg.title
		m.state = viewScrollable
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Si estamos en vista scrollable, manejar scroll
	if m.state == viewScrollable {
		switch msg.String() {
		case "q", "esc", "enter":
			m.viewportReady = false
			m.state = viewMainMenu
			m.cursor = 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		if m.streaming {
			m.stopStreaming()
			m.streaming = false
			m.state = viewMainMenu
			m.cursor = 0
			return m, nil
		}
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
		if m.state != viewLogsStream && m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.state != viewLogsStream {
			m.cursor = m.incrementCursor()
		}
		return m, nil

	case "enter", " ":
		return m.handleSelect()

	case "esc":
		if m.streaming {
			m.stopStreaming()
			m.streaming = false
			m.state = viewMainMenu
			m.cursor = 0
			return m, nil
		}
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

func (m *Model) stopStreaming() {
	if m.streamStopCh != nil {
		close(m.streamStopCh)
		m.streamStopCh = nil
	}
}

func (m Model) incrementCursor() int {
	max := 0
	switch m.state {
	case viewMainMenu:
		max = len(mainMenuItems) - 1
	case viewSelectProject:
		max = len(m.projectKeys) - 1
	case viewSelectLogType:
		max = len(logTypeMenuItems) - 1
	case viewNginx:
		max = len(nginxMenuItems) - 1
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
	case viewSelectLogType:
		return m.handleLogTypeSelect()
	case viewNginx:
		return m.handleNginxSelect()
	case viewResult, viewLogs, viewStatus:
		if m.connectionError != "" {
			return m, tea.Quit
		}
		m.state = viewMainMenu
		m.cursor = 0
		return m, nil
	case viewLogsStream:
		m.stopStreaming()
		m.streaming = false
		m.state = viewMainMenu
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m Model) handleMainMenuSelect() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0: // Deploy
		m.selectedAction = "deploy"
		m.state = viewSelectProject
		m.cursor = 0
	case 1: // Logs
		m.selectedAction = "logs"
		m.state = viewSelectProject
		m.cursor = 0
	case 2: // Restart
		m.selectedAction = "restart"
		m.state = viewSelectProject
		m.cursor = 0
	case 3: // Status
		m.state = viewDeploying
		return m, m.getStatus()
	case 4: // Nginx
		m.state = viewNginx
		m.cursor = 0
	case 5: // Salir
		m.sshClient.Close()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleProjectSelect() (tea.Model, tea.Cmd) {
	m.selectedProject = m.projectKeys[m.cursor]
	project, _ := m.config.GetProject(m.selectedProject)

	switch m.selectedAction {
	case "deploy":
		m.state = viewDeploying
		m.logs = []string{}
		return m, m.doDeploy(project)
	case "logs":
		if project.Type != "pm2" {
			m.state = viewResult
			m.result = "Este proyecto es estatico, no tiene logs de PM2"
			m.resultSuccess = false
			return m, nil
		}
		m.state = viewSelectLogType
		m.cursor = 0
		return m, nil
	case "restart":
		if project.Type != "pm2" {
			m.state = viewResult
			m.result = "Este proyecto es estatico, no usa PM2"
			m.resultSuccess = false
			return m, nil
		}
		m.state = viewDeploying
		return m, m.doRestart(project)
	}
	return m, nil
}

func (m Model) handleLogTypeSelect() (tea.Model, tea.Cmd) {
	project, _ := m.config.GetProject(m.selectedProject)

	switch m.cursor {
	case 0: // Tiempo real
		m.logBuffer = &logBuffer{lines: []string{}}
		m.logs = []string{}
		m.state = viewLogsStream
		m.streaming = true
		m.streamPm2Name = project.PM2Name
		return m, tea.Batch(m.startLogStream(project.PM2Name), streamTick())
	case 1: // Ultimas 100 lineas
		m.state = viewDeploying
		return m, m.getLogs(project.PM2Name)
	case 2: // Volver
		m.state = viewMainMenu
		m.cursor = 0
	}
	return m, nil
}

func (m Model) handleNginxSelect() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0: // Ver configuracion
		m.state = viewDeploying
		return m, m.getNginxConfig()
	case 1: // Copiar config al clipboard
		m.state = viewDeploying
		return m, m.nginxCopyToClipboard()
	case 2: // Test
		m.state = viewDeploying
		return m, m.nginxTest()
	case 3: // Reload
		m.state = viewDeploying
		return m, m.nginxReload()
	case 4: // Volver
		m.state = viewMainMenu
		m.cursor = 0
	}
	return m, nil
}

// Commands
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

func (m Model) doRestart(project config.Project) tea.Cmd {
	return func() tea.Msg {
		exec := executor.New(project, m.sshClient)
		_, err := exec.Restart()
		if err != nil {
			return deployDoneMsg{success: false, message: fmt.Sprintf("error: %v", err)}
		}
		return deployDoneMsg{success: true, message: fmt.Sprintf("%s reiniciado", project.Name)}
	}
}

func (m Model) getLogs(pm2Name string) tea.Cmd {
	return func() tea.Msg {
		logs, err := executor.GetPM2Logs(m.sshClient, pm2Name, 100)
		return logsMsg{logs: logs, err: err}
	}
}

func (m *Model) startLogStream(pm2Name string) tea.Cmd {
	m.streamStopCh = make(chan struct{})
	stopCh := m.streamStopCh
	sshClient := m.sshClient
	buffer := m.logBuffer

	return func() tea.Msg {
		outputCh := make(chan string, 100)

		cmd := fmt.Sprintf("pm2 logs %s --raw --lines 20", pm2Name)
		err := sshClient.RunStream(cmd, outputCh, stopCh)
		if err != nil {
			buffer.append(fmt.Sprintf("Error: %v", err))
			return nil
		}

		go func() {
			for {
				select {
				case <-stopCh:
					return
				case line, ok := <-outputCh:
					if !ok {
						return
					}
					for _, l := range strings.Split(line, "\n") {
						if l != "" {
							buffer.append(l)
						}
					}
				}
			}
		}()

		return nil
	}
}

func (m Model) getStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := executor.GetPM2Status(m.sshClient)
		return statusMsg{status: status, err: err}
	}
}

func (m Model) nginxTest() tea.Cmd {
	return func() tea.Msg {
		output, err := executor.NginxTest(m.sshClient)
		return nginxMsg{output: output, success: err == nil}
	}
}

func (m Model) nginxReload() tea.Cmd {
	return func() tea.Msg {
		output, err := executor.NginxReload(m.sshClient)
		return nginxMsg{output: output, success: err == nil}
	}
}

func (m Model) getNginxConfig() tea.Cmd {
	return func() tea.Msg {
		output, err := executor.GetNginxConfig(m.sshClient)
		return scrollableContentMsg{
			title:   "Configuracion Nginx",
			content: output,
			err:     err,
		}
	}
}

func (m Model) nginxCopyToClipboard() tea.Cmd {
	return func() tea.Msg {
		output, err := executor.NginxCopyConfig(m.sshClient)
		return nginxMsg{output: output, success: err == nil}
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
	case viewSelectLogType:
		s.WriteString(m.renderLogTypeMenu())
	case viewDeploying:
		s.WriteString(m.renderDeploying())
	case viewLogs:
		s.WriteString(m.renderLogs())
	case viewLogsStream:
		s.WriteString(m.renderLogsStream())
	case viewNginx:
		s.WriteString(m.renderNginxMenu())
	case viewResult:
		s.WriteString(m.renderResult())
	case viewScrollable:
		s.WriteString(m.renderScrollable())
	}

	var help string
	if m.state == viewScrollable {
		help = helpStyle.Render("up/down/PgUp/PgDn: scroll | esc/q: volver")
	} else {
		help = helpStyle.Render("up/down: navegar | enter: seleccionar | esc/q: volver | ctrl+c: salir")
	}
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

var menuIcons = []string{"◈", "◉", "◎", "◐", "◆", "◇"}

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

func (m Model) renderLogTypeMenu() string {
	var s strings.Builder
	project, _ := m.config.GetProject(m.selectedProject)
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("  logs: %s", project.Name)))
	s.WriteString("\n\n")

	logIcons := []string{"◉", "◎", "◁"}
	for i, item := range logTypeMenuItems {
		icon := logIcons[i%len(logIcons)]
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

func (m Model) renderDeploying() string {
	return fmt.Sprintf("%s ejecutando...\n", m.spinner.View())
}

func (m Model) renderLogs() string {
	var s strings.Builder
	s.WriteString(titleStyle.Render("logs"))
	s.WriteString("\n\n")

	maxLines := m.height - 9
	if maxLines < 5 {
		maxLines = 5
	}
	if maxLines > 50 {
		maxLines = 50
	}

	maxWidth := m.width - 4
	if maxWidth < 40 {
		maxWidth = 40
	}

	start := 0
	if len(m.logs) > maxLines {
		start = len(m.logs) - maxLines
	}

	for _, log := range m.logs[start:] {
		if len(log) > maxWidth {
			log = log[:maxWidth-3] + "..."
		}
		s.WriteString(logStyle.Render(log))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("enter o esc para volver"))

	return s.String()
}

func (m Model) renderLogsStream() string {
	var s strings.Builder
	project, _ := m.config.GetProject(m.selectedProject)

	s.WriteString(titleStyle.Render(project.Name))
	s.WriteString(" ")
	s.WriteString(errorStyle.Render(IconLive + " live"))
	s.WriteString("\n\n")

	maxLines := m.height - 9
	if maxLines < 5 {
		maxLines = 5
	}
	if maxLines > 50 {
		maxLines = 50
	}

	maxWidth := m.width - 4
	if maxWidth < 40 {
		maxWidth = 40
	}

	start := 0
	if len(m.logs) > maxLines {
		start = len(m.logs) - maxLines
	}

	if len(m.logs) == 0 {
		s.WriteString(mutedStyle.Render("  conectando al stream...\n"))
	} else {
		for _, log := range m.logs[start:] {
			if len(log) > maxWidth {
				log = log[:maxWidth-3] + "..."
			}
			s.WriteString(logStyle.Render(log))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("esc para detener %s %d lineas", IconDot, len(m.logs))))

	return s.String()
}

func (m Model) renderNginxMenu() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("  nginx operations"))
	s.WriteString("\n\n")

	nginxIcons := []string{"◈", "◇", "◎", "◉", "◁"}
	for i, item := range nginxMenuItems {
		icon := nginxIcons[i%len(nginxIcons)]
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

func (m Model) renderScrollable() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render(m.viewportTitle))
	s.WriteString("\n")

	scrollInfo := fmt.Sprintf("%d/%d %.0f%%",
		m.viewport.YOffset+1,
		m.viewport.TotalLineCount(),
		m.viewport.ScrollPercent()*100)
	s.WriteString(mutedStyle.Render(scrollInfo))
	s.WriteString("\n\n")

	s.WriteString(m.viewport.View())

	return s.String()
}
