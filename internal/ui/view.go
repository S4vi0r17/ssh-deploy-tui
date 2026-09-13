package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	s.WriteString(subtitleStyle.Render("  " + m.busyLabel))
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
		s.WriteString("  ")
		s.WriteString(mark)
		s.WriteString(" ")
		s.WriteString(normalStyle.Render(st.name))
		s.WriteString("\n")
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
		return "enter restart" + d + "R all" + d + "s status" + d + nav
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
