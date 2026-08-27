package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	maxFrameWidth = 92
	minFrameWidth = 46
	// WHY: alto del cromo fijo (header 3 + tabs 2 + separador 1 + help 2) para
	// que el cuerpo tenga siempre la misma altura y nada salte entre pantallas.
	chromeHeight = 8
	// Ancho mínimo para la barra de pestañas dibujada; abajo de eso se usa la compacta.
	fancyTabsMinWidth = 62
)

func (m Model) frameWidth() int {
	w := m.width - 4
	if m.width == 0 {
		return 80
	}
	if w > maxFrameWidth {
		w = maxFrameWidth
	}
	if w < minFrameWidth {
		w = minFrameWidth
	}
	return w
}

func (m Model) bodyHeight() int {
	h := m.height - chromeHeight
	if h < 8 {
		h = 8
	}
	return h
}

func (m Model) renderHeader(width int) string {
	appName := m.config.AppName
	if appName == "" {
		appName = "SSH Deploy"
	}
	title := titleStyle.Render(IconTerminal + " " + appName)

	var status string
	switch {
	case m.sshClient.IsConnected():
		status = statusOnlineStyle.Render(IconFilled + " " + m.sshClient.GetHost())
	case m.state == viewConnecting:
		status = mutedStyle.Render(IconCircle + " connecting…")
	default:
		status = statusOfflineStyle.Render(IconCircle + " offline")
	}

	inner := width - 6 // 2 de borde + 4 de padding
	gap := inner - lipgloss.Width(title) - lipgloss.Width(status)
	if gap < 1 {
		gap = 1
	}

	return headerBoxStyle.Width(width - 2).Render(title + strings.Repeat(" ", gap) + status)
}

func (m Model) renderTabBar(width int) string {
	if width < fancyTabsMinWidth {
		return m.renderTabBarCompact(width)
	}

	var top, bottom strings.Builder
	top.WriteString(" ")
	bottom.WriteString(tabRuleStyle.Render("─"))
	used := 1

	for i, t := range tabs {
		if tabID(i) == m.activeTab {
			top.WriteString(tabActiveEdgeStyle.Render("╭─ ") + tabActiveStyle.Render(t.name) + tabActiveEdgeStyle.Render(" ─╮"))
			bottom.WriteString(tabActiveEdgeStyle.Render("┘") + strings.Repeat(" ", len(t.name)+4) + tabActiveEdgeStyle.Render("└"))
		} else {
			top.WriteString("   " + tabInactiveStyle.Render(t.name) + "   ")
			bottom.WriteString(tabRuleStyle.Render(strings.Repeat("─", len(t.name)+6)))
		}
		used += len(t.name) + 6
	}

	if fill := width - used; fill > 0 {
		bottom.WriteString(tabRuleStyle.Render(strings.Repeat("─", fill)))
	}

	return top.String() + "\n" + bottom.String()
}

func (m Model) renderTabBarCompact(width int) string {
	var top strings.Builder
	top.WriteString(" ")
	for i, t := range tabs {
		if tabID(i) == m.activeTab {
			top.WriteString(tabActiveStyle.Render("[" + t.name + "]"))
		} else {
			top.WriteString(tabInactiveStyle.Render(" " + t.name + " "))
		}
	}
	return top.String() + "\n" + tabRuleStyle.Render(strings.Repeat("─", width))
}

type row struct {
	icon    string
	title   string
	meta    string
	desc    string
	badge   string
	badgeOn bool
}

// renderRows dibuja la lista en un panel y la descripción del ítem activo
// debajo, en una línea fija: así la lista no cambia de alto al mover el cursor.
func renderRows(width int, rows []row, cursor int, empty string) string {
	inner := width - 4 // 2 de borde + 2 de padding

	var body strings.Builder
	if len(rows) == 0 {
		body.WriteString(mutedStyle.Render(empty))
	}

	for i, r := range rows {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(renderRow(inner, r, i == cursor))
	}

	panel := panelStyle.Width(width - 2).Render(body.String())

	desc := ""
	if cursor >= 0 && cursor < len(rows) {
		desc = truncate(rows[cursor].desc, width-4)
	}

	return panel + "\n" + descStyle.Width(width).Render(desc)
}

func renderRow(inner int, r row, selected bool) string {
	head, meta := rowStyle, rowMetaStyle
	if selected {
		head, meta = rowSelectedStyle, rowSelectedMetaStyle
	}

	lead := "    "
	if selected {
		lead = "  " + IconArrow + " "
	}
	label := lead + r.icon + "  " + r.title

	plain := label
	if r.meta != "" {
		plain += "  " + r.meta
	}
	gap := inner - lipgloss.Width(plain) - lipgloss.Width(r.badge)
	if gap < 1 {
		gap = 1
	}

	line := head.Render(label)
	if r.meta != "" {
		line += meta.Render("  " + r.meta)
	}
	line += meta.Render(strings.Repeat(" ", gap))

	if r.badge != "" {
		badge := badgeOffStyle
		if r.badgeOn {
			badge = badgeOnStyle
		}
		if selected {
			badge = badge.Background(surface0)
		}
		line += badge.Render(r.badge)
	}

	return line
}

func truncate(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}
