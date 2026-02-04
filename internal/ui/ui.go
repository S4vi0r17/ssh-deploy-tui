package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewSplash viewState = iota
	viewMainMenu
)

const AppVersion = "1.0.0"

type Model struct {
	state      viewState
	spinner    spinner.Model
	width      int
	height     int
	splashTick int
}

type splashTickMsg struct{}

func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		state:      viewSplash,
		spinner:    s,
		splashTick: 0,
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		if m.state == viewSplash {
			m.state = viewMainMenu
			return m, nil
		}

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
	}

	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	switch m.state {
	case viewSplash:
		return m.renderSplash()
	case viewMainMenu:
		s.WriteString(titleStyle.Render("SSH Deploy TUI"))
		s.WriteString("\n\n")
		s.WriteString(normalStyle.Render("Menú principal - próximamente..."))
	}

	help := helpStyle.Render("q: salir • ctrl+c: salir")
	s.WriteString("\n\n")
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
		subtitle = "Cloud Remote Server • Deploy System"
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
