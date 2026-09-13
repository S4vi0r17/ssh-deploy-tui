package ui

import (
	"fmt"
	"strings"

	"sdt/internal/config"
	"sdt/internal/executor"
	"sdt/internal/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

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
			fmt.Fprintf(&sb, "deploy of %s\n\n", project.Name)
			for _, r := range results {
				if r.Success {
					fmt.Fprintf(&sb, "  %s %s\n", IconCheck, r.Step)
				} else {
					fmt.Fprintf(&sb, "  %s %s: %s\n", IconCross, r.Step, r.Error)
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

// WHY: sigue con el resto si uno falla, para que una app caida no bloquee las demas.
func restartAllRunner(projects []config.Project, sshClient *ssh.Client, ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			var sb strings.Builder
			sb.WriteString("restart all\n\n")
			failed := 0

			for _, project := range projects {
				ch <- deployStepMsg{name: project.Name, status: stepRunning}

				if _, err := executor.New(project, sshClient).Restart(); err != nil {
					failed++
					ch <- deployStepMsg{name: project.Name, status: stepFailed}
					fmt.Fprintf(&sb, "  %s %s: %v\n", IconCross, project.Name, err)
					continue
				}

				ch <- deployStepMsg{name: project.Name, status: stepDone}
				fmt.Fprintf(&sb, "  %s %s\n", IconCheck, project.Name)
			}

			ch <- deployDoneMsg{success: failed == 0, message: sb.String()}
		}()
		return nil
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
