package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Logo ASCII - S4v!0r (Savior in leet speak)
const Logo = `
░██████╗░░██╗██╗██╗░░░██╗██╗░█████╗░██████╗░
██╔════╝░██╔╝██║██║░░░██║██║██╔══██╗██╔══██╗
╚█████╗░██╔╝░██║╚██╗░██╔╝██║██║░░██║██████╔╝
░╚═══██╗███████║░╚████╔╝░╚═╝██║░░██║██╔══██╗
██████╔╝╚════██║░░╚██╔╝░░██╗╚█████╔╝██║░░██║
╚═════╝░░░░░░╚═╝░░░╚═╝░░░╚═╝░╚════╝░╚═╝░░╚═╝`

var LogoClean = strings.ReplaceAll(Logo, "░", " ")

// Paleta Catppuccin Mocha
var (
	rosewater = lipgloss.Color("#f5e0dc")
	flamingo  = lipgloss.Color("#f2cdcd")
	pink      = lipgloss.Color("#f5c2e7")
	mauve     = lipgloss.Color("#cba6f7")
	red       = lipgloss.Color("#f38ba8")
	green     = lipgloss.Color("#a6e3a1")
	lavender  = lipgloss.Color("#b4befe")

	text     = lipgloss.Color("#cdd6f4")
	subtext0 = lipgloss.Color("#a6adc8")
	overlay1 = lipgloss.Color("#7f849c")
	overlay0 = lipgloss.Color("#6c7086")
	surface2 = lipgloss.Color("#585b70")
	surface1 = lipgloss.Color("#45475a")
	surface0 = lipgloss.Color("#313244")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lavender)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(overlay1).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(mauve).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(text)

	mutedStyle = lipgloss.NewStyle().
			Foreground(overlay0)

	successStyle = lipgloss.NewStyle().
			Foreground(green)

	errorStyle = lipgloss.NewStyle().
			Foreground(red)

	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(surface2).
			Foreground(text).
			Padding(0, 2)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lavender)

	logStyle = lipgloss.NewStyle().
			Foreground(subtext0)

	helpStyle = lipgloss.NewStyle().
			Foreground(overlay0).
			MarginTop(1).
			PaddingLeft(2)

	logoGradientStyle1 = lipgloss.NewStyle().
				Foreground(pink).
				Bold(true)

	logoGradientStyle2 = lipgloss.NewStyle().
				Foreground(flamingo)

	logoGradientStyle3 = lipgloss.NewStyle().
				Foreground(rosewater)

	splashSubtitleStyle = lipgloss.NewStyle().
				Foreground(overlay1).
				Italic(true).
				MarginTop(1)

	versionBadgeStyle = lipgloss.NewStyle().
				Foreground(surface0).
				Background(mauve).
				Padding(0, 1).
				Bold(true)

	accentLineStyle = lipgloss.NewStyle().
			Foreground(surface2)

	statusOnlineStyle = lipgloss.NewStyle().
				Foreground(green).
				Bold(true)

	statusOfflineStyle = lipgloss.NewStyle().
				Foreground(red)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(mauve).
			Bold(true)

	tabActiveEdgeStyle = lipgloss.NewStyle().
				Foreground(mauve)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(overlay0)

	tabRuleStyle = lipgloss.NewStyle().
			Foreground(surface1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(surface1).
			Padding(0, 1)

	// WHY: cada fragmento lleva su Background porque lipgloss no lo propaga a texto ya estilado.
	rowSelectedStyle = lipgloss.NewStyle().
				Foreground(mauve).
				Background(surface0).
				Bold(true)

	rowSelectedMetaStyle = lipgloss.NewStyle().
				Foreground(subtext0).
				Background(surface0)

	rowStyle = lipgloss.NewStyle().
			Foreground(text)

	rowMetaStyle = lipgloss.NewStyle().
			Foreground(overlay0)

	descStyle = lipgloss.NewStyle().
			Foreground(overlay1).
			Italic(true).
			PaddingLeft(2)

	badgeOnStyle = lipgloss.NewStyle().
			Foreground(green)

	badgeOffStyle = lipgloss.NewStyle().
			Foreground(overlay0)
)

const (
	IconArrow    = "›"
	IconDot      = "•"
	IconCheck    = "✓"
	IconCross    = "×"
	IconCircle   = "○"
	IconFilled   = "●"
	IconFolder   = "□"
	IconServer   = "◆"
	IconLive     = "●"
	IconTerminal = "❯"
	IconSSH      = "⌁"
)
