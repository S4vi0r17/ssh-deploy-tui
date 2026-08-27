package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sdt/internal/config"
	"sdt/internal/executor"
	"sdt/internal/ssh"
	"sdt/internal/tunnel"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewSplash viewState = iota
	viewConnecting
	viewTab
	viewBusy
	viewLogs
	viewLogsStream
	viewResult
	viewScrollable
)

type tabID int

const (
	tabDeploy tabID = iota
	tabLogs
	tabPM2
	tabTunnels
	tabNginx
)

var tabs = []struct{ name string }{
	{name: "Deploy"},
	{name: "Logs"},
	{name: "PM2"},
	{name: "Tunnels"},
	{name: "Nginx"},
}

const AppVersion = "2.0.0"

type menuItem struct {
	title       string
	description string
}

var nginxActions = []menuItem{
	{title: "View config", description: "Show sites-available in a scrollable panel"},
	{title: "Copy config", description: "Copy the config to the clipboard"},
	{title: "Test config", description: "nginx -t: verify the syntax"},
	{title: "Reload", description: "Reload nginx without dropping connections"},
}

// WHY: written by the stream goroutine, read by the UI tick — needs a mutex.
type logBuffer struct {
	lines []string
	mu    sync.Mutex
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
	sshClient         *ssh.Client
	deployChan        chan tea.Msg
	config            *config.Config
	logBuffer         *logBuffer
	streamStopCh      chan struct{}
	connectionError   string
	viewportTitle     string
	selectedProject   string
	result            string
	logs              []string
	tunnels           []*tunnel.Tunnel
	projectKeys       []string
	pm2Keys           []string
	deploySteps       []deployStep
	cursors           []int
	portInput         textinput.Model
	viewport          viewport.Model
	spinner           spinner.Model
	width             int
	height            int
	state             viewState
	activeTab         tabID
	splashTick        int
	resultSuccess     bool
	editingTunnelPort bool
	logsFollow        bool
	viewportReady     bool
	streaming         bool
}

const (
	stepRunning = iota
	stepDone
	stepFailed
)

type deployStep struct {
	name   string
	status int
}

type sshConnectedMsg struct{ err error }
type deployDoneMsg struct {
	message string
	success bool
}
type deployStepMsg struct {
	name   string
	status int
}
type logsMsg struct {
	err  error
	logs []string
}
type streamTickMsg struct{}
type nginxMsg struct {
	output  string
	success bool
}
type scrollableContentMsg struct {
	err     error
	title   string
	content string
}
type splashTickMsg struct{}
type tunnelResultMsg struct {
	err error
}

func NewModel(cfg *config.Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	// WHY: GetProjectList recorre un map, así que sin ordenar las filas cambian
	// de posición entre ejecuciones y el cursor deja de ser predecible.
	keys := cfg.GetProjectList()
	sort.Strings(keys)

	var pm2Keys []string
	for _, k := range keys {
		if p, _ := cfg.GetProject(k); p.Type == "pm2" {
			pm2Keys = append(pm2Keys, k)
		}
	}

	return Model{
		config:      cfg,
		sshClient:   ssh.NewClient(&cfg.SSH),
		state:       viewSplash,
		spinner:     s,
		projectKeys: keys,
		pm2Keys:     pm2Keys,
		cursors:     make([]int, len(tabs)),
		logBuffer:   &logBuffer{lines: []string{}},
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
			m.viewport.Width, m.viewport.Height = m.viewportSize()
			if m.state == viewLogs || m.state == viewLogsStream {
				m.refreshLogsViewport()
			}
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
			m.result = fmt.Sprintf("SSH connection error:\n\n%s\n\nCheck your config.yaml", msg.err.Error())
			m.resultSuccess = false
			return m, nil
		}
		m.tunnels = make([]*tunnel.Tunnel, len(m.config.Tunnels))
		for i, tc := range m.config.Tunnels {
			m.tunnels[i] = tunnel.New(tc.Name, tc.LocalPort, tc.RemoteHost, tc.RemotePort, m.sshClient)
			if tc.AutoStart {
				m.tunnels[i].Start()
			}
		}
		m.state = viewTab
		return m, nil

	case deployStepMsg:
		found := false
		for i := range m.deploySteps {
			if m.deploySteps[i].name == msg.name {
				m.deploySteps[i].status = msg.status
				found = true
				break
			}
		}
		if !found {
			m.deploySteps = append(m.deploySteps, deployStep{name: msg.name, status: msg.status})
		}
		// WHY: re-arm so the next step or the final result is also picked up.
		return m, waitForDeploy(m.deployChan)

	case deployDoneMsg:
		m.state = viewResult
		m.result = msg.message
		m.resultSuccess = msg.success
		m.deploySteps = nil
		return m, nil

	case logsMsg:
		m.state = viewLogs
		if msg.err != nil {
			m.logs = []string{fmt.Sprintf("Error: %v", msg.err)}
		} else {
			m.logs = msg.logs
		}
		w, h := m.viewportSize()
		m.viewport = viewport.New(w, h)
		m.viewportReady = true
		m.logsFollow = true
		m.refreshLogsViewport()
		return m, nil

	case streamTickMsg:
		if m.streaming && m.state == viewLogsStream {
			m.logs = m.logBuffer.getAll()
			m.refreshLogsViewport()
			return m, streamTick()
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
		w, h := m.viewportSize()
		m.viewport = viewport.New(w, h)
		m.viewport.SetContent(logStyle.Width(w).Render(msg.content))
		m.viewportReady = true
		m.viewportTitle = msg.title
		m.state = viewScrollable
		return m, nil

	case tunnelResultMsg:
		if msg.err != nil {
			m.state = viewResult
			m.result = fmt.Sprintf("Error al iniciar túnel:\n\n%v", msg.err)
			m.resultSuccess = false
			return m, nil
		}
		m.state = viewTab
		return m, nil

	default:
		if m.editingTunnelPort {
			var cmd tea.Cmd
			m.portInput, cmd = m.portInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return m.quit()
	}

	if m.editingTunnelPort {
		switch key {
		case "esc":
			m.editingTunnelPort = false
			return m, nil
		case "enter":
			return m.applyTunnelPortEdit()
		default:
			var cmd tea.Cmd
			m.portInput, cmd = m.portInput.Update(msg)
			return m, cmd
		}
	}

	switch m.state {
	case viewConnecting, viewBusy:
		return m, nil

	case viewResult:
		switch key {
		case "enter", "esc", "q":
			if m.connectionError != "" {
				return m.quit()
			}
			m.state = viewTab
		}
		return m, nil

	case viewLogs, viewLogsStream, viewScrollable:
		if key == "esc" || key == "q" {
			if m.streaming {
				m.stopStreaming()
				m.streaming = false
			}
			m.viewportReady = false
			m.state = viewTab
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.logsFollow = m.viewport.AtBottom()
		return m, cmd
	}

	return m.handleTabKey(key)
}

func (m Model) handleTabKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m.quit()

	case "up", "k":
		if c := m.cursor(); c > 0 {
			m.setCursor(c - 1)
		}

	case "down", "j":
		if c := m.cursor(); c < m.rowCount()-1 {
			m.setCursor(c + 1)
		}

	case "tab", "right":
		m.activeTab = (m.activeTab + 1) % tabID(len(tabs))

	case "shift+tab", "left":
		m.activeTab = (m.activeTab + tabID(len(tabs)) - 1) % tabID(len(tabs))

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := int(key[0] - '1'); i < len(tabs) {
			m.activeTab = tabID(i)
		}

	case "enter", " ":
		return m.handleEnter()

	case "l":
		if m.activeTab == tabLogs && m.rowCount() > 0 {
			return m.showLastLogs(m.pm2Keys[m.cursor()])
		}

	case "s":
		if m.activeTab == tabPM2 {
			m.state = viewBusy
			return m, m.getStatus()
		}

	case "e":
		if m.activeTab == tabTunnels && m.rowCount() > 0 {
			return m.startTunnelPortEdit()
		}
	}

	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.rowCount() == 0 {
		return m, nil
	}
	c := m.cursor()

	switch m.activeTab {
	case tabDeploy:
		return m.startDeploy(m.projectKeys[c])
	case tabLogs:
		return m.startLiveLogs(m.pm2Keys[c])
	case tabPM2:
		return m.restartProject(m.pm2Keys[c])
	case tabTunnels:
		return m.toggleTunnel(c)
	case tabNginx:
		return m.runNginxAction(c)
	}
	return m, nil
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	for _, t := range m.tunnels {
		t.Stop()
	}
	m.stopStreaming()
	m.sshClient.Close()
	return m, tea.Quit
}

func (m *Model) stopStreaming() {
	if m.streamStopCh != nil {
		close(m.streamStopCh)
		m.streamStopCh = nil
	}
}

func (m Model) rowCount() int {
	switch m.activeTab {
	case tabDeploy:
		return len(m.projectKeys)
	case tabLogs, tabPM2:
		return len(m.pm2Keys)
	case tabTunnels:
		return len(m.tunnels)
	case tabNginx:
		return len(nginxActions)
	}
	return 0
}

// WHY: el cursor vive por pestaña y las filas de Tunnels aparecen recién tras
// conectar, así que se acota en la lectura en vez de al cambiar de pestaña.
func (m Model) cursor() int {
	c := m.cursors[m.activeTab]
	if n := m.rowCount(); c >= n {
		c = n - 1
	}
	if c < 0 {
		c = 0
	}
	return c
}

func (m *Model) setCursor(c int) {
	m.cursors[m.activeTab] = c
}

func (m Model) startDeploy(key string) (tea.Model, tea.Cmd) {
	project, _ := m.config.GetProject(key)
	m.selectedProject = key
	m.state = viewBusy
	m.logs = nil
	m.deploySteps = nil
	m.deployChan = make(chan tea.Msg, 64)
	return m, tea.Batch(
		deployRunner(project, m.sshClient, m.deployChan),
		waitForDeploy(m.deployChan),
	)
}

func (m Model) startLiveLogs(key string) (tea.Model, tea.Cmd) {
	project, _ := m.config.GetProject(key)
	m.selectedProject = key
	m.logBuffer = &logBuffer{lines: []string{}}
	m.logs = nil
	m.state = viewLogsStream
	m.streaming = true
	m.logsFollow = true

	w, h := m.viewportSize()
	m.viewport = viewport.New(w, h)
	m.viewportReady = true
	m.refreshLogsViewport()

	return m, tea.Batch(m.startLogStream(project.PM2Name), streamTick())
}

func (m Model) showLastLogs(key string) (tea.Model, tea.Cmd) {
	project, _ := m.config.GetProject(key)
	m.selectedProject = key
	m.state = viewBusy
	return m, m.getLogs(project.PM2Name)
}

func (m Model) restartProject(key string) (tea.Model, tea.Cmd) {
	project, _ := m.config.GetProject(key)
	m.selectedProject = key
	m.state = viewBusy
	return m, m.doRestart(project)
}

func (m Model) toggleTunnel(idx int) (tea.Model, tea.Cmd) {
	t := m.tunnels[idx]
	sshClient := m.sshClient
	return m, func() tea.Msg {
		if t.IsActive() {
			t.Stop()
			return tunnelResultMsg{}
		}
		if err := sshClient.EnsureConnected(); err != nil {
			return tunnelResultMsg{err: fmt.Errorf("conexion SSH caida, no se pudo reconectar: %v", err)}
		}
		return tunnelResultMsg{err: t.Start()}
	}
}

func (m Model) runNginxAction(idx int) (tea.Model, tea.Cmd) {
	m.state = viewBusy
	switch idx {
	case 0:
		return m, m.getNginxConfig()
	case 1:
		return m, m.nginxCopyToClipboard()
	case 2:
		return m, m.nginxTest()
	case 3:
		return m, m.nginxReload()
	}
	return m, nil
}

func (m Model) startTunnelPortEdit() (tea.Model, tea.Cmd) {
	t := m.tunnels[m.cursor()]

	ti := textinput.New()
	ti.Placeholder = strconv.Itoa(t.Port())
	ti.CharLimit = 5
	ti.Width = 10
	cmd := ti.Focus()

	m.portInput = ti
	m.editingTunnelPort = true
	return m, cmd
}

func (m Model) applyTunnelPortEdit() (tea.Model, tea.Cmd) {
	port, err := strconv.Atoi(strings.TrimSpace(m.portInput.Value()))
	if err != nil || port < 1 || port > 65535 {
		// WHY: no error UI yet, so just stay in edit mode to let it be fixed.
		return m, nil
	}
	m.editingTunnelPort = false

	t := m.tunnels[m.cursor()]
	wasActive := t.IsActive()
	t.SetLocalPort(port)
	sshClient := m.sshClient

	return m, func() tea.Msg {
		if !wasActive {
			return tunnelResultMsg{}
		}
		t.Stop()
		if err := sshClient.EnsureConnected(); err != nil {
			return tunnelResultMsg{err: fmt.Errorf("conexion SSH caida, no se pudo reconectar: %v", err)}
		}
		return tunnelResultMsg{err: t.Start()}
	}
}

// WHY: runs the deploy in its own goroutine and pushes each step (and the
// final result) to ch; the UI drains it via waitForDeploy.
func deployRunner(project config.Project, sshClient *ssh.Client, ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			exec := executor.New(project, sshClient)
			progress := make(chan executor.StepProgress, 64)
			errCh := make(chan error, 1)

			go func() {
				errCh <- exec.Deploy(progress)
				close(progress)
			}()

			for p := range progress {
				status := stepRunning
				if p.Failed {
					status = stepFailed
				} else if p.Done {
					status = stepDone
				}
				ch <- deployStepMsg{name: p.Name, status: status}
			}

			err := <-errCh
			if err != nil {
				ch <- deployDoneMsg{success: false, message: fmt.Sprintf("Error: %v", err)}
				return
			}

			results := exec.GetResults()
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("deploy of %s\n\n", project.Name))
			for _, r := range results {
				if r.Success {
					sb.WriteString(fmt.Sprintf("  %s %s\n", IconCheck, r.Step))
				} else {
					sb.WriteString(fmt.Sprintf("  %s %s: %s\n", IconCross, r.Step, r.Error))
				}
			}
			ch <- deployDoneMsg{success: true, message: sb.String()}
		}()
		return nil
	}
}

func waitForDeploy(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m Model) doRestart(project config.Project) tea.Cmd {
	return func() tea.Msg {
		exec := executor.New(project, m.sshClient)
		_, err := exec.Restart()
		if err != nil {
			return deployDoneMsg{success: false, message: fmt.Sprintf("error: %v", err)}
		}
		return deployDoneMsg{success: true, message: fmt.Sprintf("%s restarted", project.Name)}
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
		return scrollableContentMsg{title: "PM2 status", content: status, err: err}
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
			title:   "Nginx configuration",
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

func (m Model) View() string {
	if m.state == viewSplash {
		return m.renderSplash()
	}

	width := m.frameWidth()
	body := lipgloss.NewStyle().
		Width(width).
		Height(m.bodyHeight()).
		MaxHeight(m.bodyHeight()).
		Render(m.renderBody(width))

	frame := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(width),
		m.renderTabBar(width),
		"",
		body,
		helpStyle.Width(width).Render(m.helpText()),
	)

	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, frame)
}

func (m Model) renderBody(width int) string {
	switch m.state {
	case viewConnecting:
		return m.renderConnecting()

	case viewBusy:
		return m.renderBusy()

	case viewResult:
		return m.renderResult(width)

	case viewLogs:
		project, _ := m.config.GetProject(m.selectedProject)
		title := titleStyle.Render(project.Name) + mutedStyle.Render(" · last 100 lines")
		return m.renderViewportPanel(width, title, m.scrollStatus())

	case viewLogsStream:
		project, _ := m.config.GetProject(m.selectedProject)
		title := titleStyle.Render(project.Name) + " " + errorStyle.Render(IconLive+" live")
		follow := "following"
		if !m.logsFollow {
			follow = "paused · ↓ to resume"
		}
		status := subtitleStyle.Render(fmt.Sprintf("%d lines %s %s", len(m.logs), IconDot, follow))
		return m.renderViewportPanel(width, title, status)

	case viewScrollable:
		return m.renderViewportPanel(width, titleStyle.Render(m.viewportTitle), m.scrollStatus())

	case viewTab:
		if m.editingTunnelPort {
			return m.renderTunnelPortEdit(width)
		}
		return m.renderTabContent(width)
	}

	return ""
}

func (m Model) renderTabContent(width int) string {
	switch m.activeTab {
	case tabDeploy:
		return renderRows(width, m.projectRows(m.projectKeys), m.cursor(), "no projects in config.yaml")
	case tabLogs, tabPM2:
		return renderRows(width, m.projectRows(m.pm2Keys), m.cursor(), "no pm2 projects in config.yaml")
	case tabTunnels:
		return renderRows(width, m.tunnelRows(), m.cursor(), "no tunnels in config.yaml")
	case tabNginx:
		return renderRows(width, nginxRows(), m.cursor(), "")
	}
	return ""
}

func (m Model) projectRows(keys []string) []row {
	rows := make([]row, 0, len(keys))
	for _, key := range keys {
		project, _ := m.config.GetProject(key)
		icon := IconFolder
		if project.Type == "pm2" {
			icon = IconServer
		}
		rows = append(rows, row{
			icon:  icon,
			title: project.Name,
			meta:  project.Branch,
			desc:  project.Path,
		})
	}
	return rows
}

func (m Model) tunnelRows() []row {
	rows := make([]row, 0, len(m.tunnels))
	for i, t := range m.tunnels {
		cfg := m.config.Tunnels[i]

		port := ":auto"
		if t.Port() != 0 {
			port = fmt.Sprintf(":%d", t.Port())
		}

		r := row{
			icon:  IconSSH,
			title: cfg.Name,
			meta:  fmt.Sprintf("%s → %s:%d", port, cfg.RemoteHost, cfg.RemotePort),
			badge: IconCircle + " inactive",
			desc:  "enter to activate · e to change the local port",
		}
		if t.IsActive() {
			r.badge = IconFilled + " active"
			r.badgeOn = true
			r.desc = "enter to deactivate · e to change the local port"
		}
		rows = append(rows, r)
	}
	return rows
}

func nginxRows() []row {
	icons := []string{"◈", "◇", "◎", "◉"}
	rows := make([]row, len(nginxActions))
	for i, action := range nginxActions {
		rows[i] = row{
			icon:  icons[i%len(icons)],
			title: action.title,
			desc:  action.description,
		}
	}
	return rows
}

func (m Model) viewportSize() (width, height int) {
	width = m.frameWidth() - 4
	if width < 20 {
		width = 20
	}
	// WHY: el panel come 2 líneas de borde y el título y el estado una cada uno.
	height = m.bodyHeight() - 4
	if height < 4 {
		height = 4
	}
	return width, height
}

func (m *Model) refreshLogsViewport() {
	if !m.viewportReady {
		return
	}

	var content string
	switch {
	case len(m.logs) > 0:
		content = logStyle.Width(m.viewport.Width).Render(strings.Join(m.logs, "\n"))
	case m.streaming:
		content = mutedStyle.Render("connecting to stream…")
	default:
		content = mutedStyle.Render("no logs")
	}

	m.viewport.SetContent(content)
	if m.logsFollow {
		m.viewport.GotoBottom()
	}
}

func (m Model) renderViewportPanel(width int, title, status string) string {
	if !m.viewportReady {
		return ""
	}
	return "  " + title + "\n" +
		panelStyle.Width(width-2).Render(m.viewport.View()) + "\n" +
		"  " + status
}

func (m Model) scrollStatus() string {
	return mutedStyle.Render(fmt.Sprintf("%d/%d · %.0f%%",
		m.viewport.YOffset+1,
		m.viewport.TotalLineCount(),
		m.viewport.ScrollPercent()*100))
}

func (m Model) renderConnecting() string {
	return "\n  " + m.spinner.View() + " Establishing SSH connection…\n" +
		"     " + mutedStyle.Render(IconSSH+" "+m.config.SSH.Host) + "\n"
}

func (m Model) renderBusy() string {
	if len(m.deploySteps) == 0 {
		return "\n  " + m.spinner.View() + " working…\n"
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("  deploying"))
	s.WriteString("\n\n")

	for _, st := range m.deploySteps {
		var mark string
		switch st.status {
		case stepDone:
			mark = successStyle.Render(IconCheck)
		case stepFailed:
			mark = errorStyle.Render(IconCross)
		default:
			mark = m.spinner.View()
		}
		s.WriteString("  " + mark + " " + normalStyle.Render(st.name) + "\n")
	}

	return s.String()
}

func (m Model) renderResult(width int) string {
	var s strings.Builder
	s.WriteString("\n")

	if m.resultSuccess {
		s.WriteString(successStyle.Render("  " + IconCheck + " operation completed"))
	} else {
		s.WriteString(errorStyle.Render("  " + IconCross + " operation failed"))
	}
	s.WriteString("\n\n")

	maxLines := m.bodyHeight() - 4
	if maxLines < 3 {
		maxLines = 3
	}

	lines := strings.Split(strings.TrimRight(m.result, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-(maxLines-1):]
		s.WriteString(mutedStyle.Render(fmt.Sprintf("  showing the last %d lines\n", len(lines))))
	}

	s.WriteString(logStyle.Width(width - 4).PaddingLeft(2).Render(strings.Join(lines, "\n")))
	return s.String()
}

func (m Model) renderTunnelPortEdit(width int) string {
	t := m.tunnels[m.cursor()]

	inner := lipgloss.JoinVertical(lipgloss.Left,
		normalStyle.Render("local port for "+t.Name),
		"",
		m.portInput.View(),
		"",
		mutedStyle.Render("valid range: 1-65535"),
	)

	return "\n" + panelStyle.Width(width-2).Render(inner)
}

func (m Model) helpText() string {
	d := " " + IconDot + " "
	switch m.state {
	case viewConnecting, viewBusy:
		return "please wait…"
	case viewResult:
		return "enter/esc back"
	case viewLogs, viewScrollable:
		return "↑↓ scroll" + d + "esc back"
	case viewLogsStream:
		return "↑↓ scroll" + d + "esc stop"
	}

	if m.editingTunnelPort {
		return "enter confirm" + d + "esc cancel"
	}

	nav := "↑↓ navigate" + d + "1-5/tab switch" + d + "q quit"
	switch m.activeTab {
	case tabDeploy:
		return "enter deploy" + d + nav
	case tabLogs:
		return "enter live" + d + "l last 100" + d + nav
	case tabPM2:
		return "enter restart" + d + "s status" + d + nav
	case tabTunnels:
		action := "activate"
		if c := m.cursor(); c < len(m.tunnels) && m.tunnels[c].IsActive() {
			action = "deactivate"
		}
		return "enter " + action + d + "e port" + d + nav
	case tabNginx:
		return "enter run" + d + nav
	}
	return nav
}

func (m Model) renderSplash() string {
	var s strings.Builder

	logoLines := strings.Split(LogoClean, "\n")
	totalHeight := len(logoLines) + 8
	topPadding := (m.height - totalHeight) / 2
	if topPadding < 0 {
		topPadding = 1
	}
	s.WriteString(strings.Repeat("\n", topPadding))

	for i, line := range logoLines {
		if line == "" {
			s.WriteString("\n")
			continue
		}
		var style lipgloss.Style
		switch pos := float64(i) / float64(len(logoLines)); {
		case pos < 0.33:
			style = logoGradientStyle1
		case pos < 0.66:
			style = logoGradientStyle2
		default:
			style = logoGradientStyle3
		}
		s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, style.Render(line)))
		s.WriteString("\n")
	}

	decorWidth := 35
	if m.width < 50 {
		decorWidth = 20
	}
	s.WriteString("\n")
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, accentLineStyle.Render(strings.Repeat("─", decorWidth))))
	s.WriteString("\n\n")

	subtitle := "Deploy System"
	if m.width >= 50 {
		subtitle = "Cloud Remote Server • Deploy System"
	}
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, splashSubtitleStyle.Render(subtitle)))
	s.WriteString("\n\n")

	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, versionBadgeStyle.Render(fmt.Sprintf(" v%s ", AppVersion))))
	s.WriteString("\n\n")

	blink := []string{"▸", "▹"}[(m.splashTick/3)%2]
	press := fmt.Sprintf("%s press any key to continue %s", blink, blink)
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, mutedStyle.Render(press)))

	return s.String()
}
