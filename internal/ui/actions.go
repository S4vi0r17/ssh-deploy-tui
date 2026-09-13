package ui

import (
	"fmt"
	"strconv"
	"strings"

	"sdt/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) startDeploy(key string) (tea.Model, tea.Cmd) {
	project, _ := m.config.GetProject(key)
	m.selectedProject = key
	m.state = viewBusy
	m.busyLabel = "deploying"
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

func (m Model) startRestartAll() (tea.Model, tea.Cmd) {
	projects := make([]config.Project, 0, len(m.pm2Keys))
	for _, key := range m.pm2Keys {
		project, _ := m.config.GetProject(key)
		projects = append(projects, project)
	}

	m.state = viewBusy
	m.busyLabel = "restarting all"
	m.deploySteps = nil
	m.deployChan = make(chan tea.Msg, 64)
	return m, tea.Batch(
		restartAllRunner(projects, m.sshClient, m.deployChan),
		waitForDeploy(m.deployChan),
	)
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
		// WHY: no hay UI de error todavía, se queda en modo edición para corregirlo.
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
