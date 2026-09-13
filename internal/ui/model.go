package ui

import (
	"sort"
	"sync"
	"time"

	"sdt/internal/config"
	"sdt/internal/ssh"
	"sdt/internal/tunnel"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	busyLabel         string
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
