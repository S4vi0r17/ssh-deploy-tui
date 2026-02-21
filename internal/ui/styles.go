package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Logo ASCII - S4v!0r (Savior en leet speak)
const Logo = `
░██████╗░░██╗██╗██╗░░░██╗██╗░█████╗░██████╗░
██╔════╝░██╔╝██║██║░░░██║██║██╔══██╗██╔══██╗
╚█████╗░██╔╝░██║╚██╗░██╔╝██║██║░░██║██████╔╝
░╚═══██╗███████║░╚████╔╝░╚═╝██║░░██║██╔══██╗
██████╔╝╚════██║░░╚██╔╝░░██╗╚█████╔╝██║░░██║
╚═════╝░░░░░░╚═╝░░░╚═╝░░░╚═╝░╚════╝░╚═╝░░╚═╝`

var LogoClean = strings.ReplaceAll(Logo, "░", "\u00A0")

const LogoSmall = `[ S4v!0r ]`
const LogoMinimal = `S4v!0r`
const LogoSimple = `S4v!0r`

// Paleta Catppuccin Mocha
var (
	rosewater = lipgloss.Color("#f5e0dc")
	flamingo  = lipgloss.Color("#f2cdcd")
	pink      = lipgloss.Color("#f5c2e7")
	mauve     = lipgloss.Color("#cba6f7")
	red       = lipgloss.Color("#f38ba8")
	maroon    = lipgloss.Color("#eba0ac")
	peach     = lipgloss.Color("#fab387")
	yellow    = lipgloss.Color("#f9e2af")
	green     = lipgloss.Color("#a6e3a1")
	teal      = lipgloss.Color("#94e2d5")
	sky       = lipgloss.Color("#89dceb")
	sapphire  = lipgloss.Color("#74c7ec")
	blue      = lipgloss.Color("#89b4fa")
	lavender  = lipgloss.Color("#b4befe")

	text     = lipgloss.Color("#cdd6f4")
	subtext1 = lipgloss.Color("#bac2de")
	subtext0 = lipgloss.Color("#a6adc8")
	overlay2 = lipgloss.Color("#9399b2")
	overlay1 = lipgloss.Color("#7f849c")
	overlay0 = lipgloss.Color("#6c7086")
	surface2 = lipgloss.Color("#585b70")
	surface1 = lipgloss.Color("#45475a")
	surface0 = lipgloss.Color("#313244")
	base     = lipgloss.Color("#1e1e2e")
	mantle   = lipgloss.Color("#181825")
	crust    = lipgloss.Color("#11111b")

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
			Foreground(subtext0).
			PaddingLeft(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(overlay0).
			MarginTop(1)

	logoStyle = lipgloss.NewStyle().
			Foreground(pink).
			Bold(true)

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

	contentBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(surface1).
			Padding(1, 2)

	headerBarStyle = lipgloss.NewStyle().
			Foreground(text).
			Background(surface0).
			Padding(0, 2).
			MarginBottom(1)
)

// Iconos minimalistas
const (
	IconArrow    = "›"
	IconDot      = "•"
	IconCheck    = "✓"
	IconCross    = "×"
	IconCircle   = "○"
	IconFilled   = "●"
	IconFolder   = "□"
	IconFile     = "◇"
	IconServer   = "◆"
	IconLive     = "●"
	IconPending  = "○"
	IconRunning  = "◐"
	IconLock     = "◈"
	IconUnlock   = "◇"
	IconTerminal = "❯"
	IconBranch   = "⎇"
	IconSSH      = "⌁"
)
