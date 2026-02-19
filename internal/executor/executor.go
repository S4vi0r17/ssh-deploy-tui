package executor

import (
	"fmt"

	"ssh-deploy-tui/internal/config"
	"ssh-deploy-tui/internal/ssh"
)

type StepResult struct {
	Step    string
	Success bool
	Output  string
	Error   string
}

type Executor struct {
	project   config.Project
	sshClient *ssh.Client
	results   []StepResult
}

func New(project config.Project, sshClient *ssh.Client) *Executor {
	return &Executor{
		project:   project,
		sshClient: sshClient,
		results:   make([]StepResult, 0),
	}
}

func (e *Executor) Deploy(outputChan chan<- string) error {
	steps := []struct {
		name string
		fn   func() (string, error)
	}{
		{"Git Pull", e.gitPull},
		{"Instalar dependencias", e.installDeps},
		{"Build", e.build},
	}

	if e.project.Type == "pm2" {
		steps = append(steps, struct {
			name string
			fn   func() (string, error)
		}{"Limpiar logs", e.flushPM2})
		steps = append(steps, struct {
			name string
			fn   func() (string, error)
		}{"Restart PM2", e.restartPM2})
	}

	for _, step := range steps {
		outputChan <- fmt.Sprintf("ejecutando %s...", step.name)

		output, err := step.fn()
		if err != nil {
			e.results = append(e.results, StepResult{
				Step:    step.name,
				Success: false,
				Output:  output,
				Error:   err.Error(),
			})
			outputChan <- fmt.Sprintf("error %s: %s", step.name, err.Error())
			return err
		}

		e.results = append(e.results, StepResult{
			Step:    step.name,
			Success: true,
			Output:  output,
		})
		outputChan <- fmt.Sprintf("completado %s", step.name)
	}

	return nil
}

func (e *Executor) gitPull() (string, error) {
	cmd := fmt.Sprintf("cd %s && git checkout %s", e.project.Path, e.project.Branch)
	_, err := e.sshClient.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("checkout: %v", err)
	}

	cmd = fmt.Sprintf("cd %s && git pull origin %s", e.project.Path, e.project.Branch)
	out, err := e.sshClient.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("pull: %v", err)
	}

	return out, nil
}

func (e *Executor) installDeps() (string, error) {
	cmd := fmt.Sprintf("cd %s && %s", e.project.Path, e.project.InstallCmd)
	out, err := e.sshClient.Run(cmd)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Executor) build() (string, error) {
	cmd := fmt.Sprintf("cd %s && %s", e.project.Path, e.project.BuildCmd)
	out, err := e.sshClient.Run(cmd)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Executor) flushPM2() (string, error) {
	out, err := e.sshClient.Run(fmt.Sprintf("pm2 flush %s", e.project.PM2Name))
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Executor) restartPM2() (string, error) {
	out, err := e.sshClient.Run(fmt.Sprintf("pm2 restart %s", e.project.PM2Name))
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Executor) GetResults() []StepResult {
	return e.results
}
