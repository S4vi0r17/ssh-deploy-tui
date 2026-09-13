package ui

import (
	"fmt"

	"sdt/internal/tunnel"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

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
		// SYNC: se re-arma para tomar también el paso siguiente o el resultado final.
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

	case "R":
		if m.activeTab == tabPM2 && m.rowCount() > 0 {
			return m.startRestartAll()
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

// WHY: las filas de Tunnels aparecen tras conectar, así que el cursor se acota al leerlo.
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
