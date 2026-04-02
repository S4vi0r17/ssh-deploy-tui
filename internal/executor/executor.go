package executor

import (
	"fmt"
	"os/exec"
	"strings"

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

func (e *Executor) Restart() (string, error) {
	if e.project.Type != "pm2" {
		return "", fmt.Errorf("proyecto estatico no usa PM2")
	}
	return e.restartPM2()
}

func (e *Executor) GetResults() []StepResult {
	return e.results
}

// GetPM2Logs obtiene los logs de PM2 via SSH
func GetPM2Logs(sshClient *ssh.Client, pm2Name string, lines int) ([]string, error) {
	out, err := sshClient.Run(fmt.Sprintf("pm2 logs %s --lines %d --nostream", pm2Name, lines))
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			logs = append(logs, line)
		}
	}
	return logs, nil
}

// GetPM2Status obtiene el estado de todos los procesos PM2 via SSH
func GetPM2Status(sshClient *ssh.Client) (string, error) {
	cmd := `pm2 jlist | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'{'ID':<4} {'NOMBRE':<22} {'STATUS':<10} {'CPU':<6} {'MEM':<10} {'UPTIME'}')
print('-' * 70)
for p in data:
    name = p.get('name', '')[:20]
    status = p.get('pm2_env', {}).get('status', 'N/A')
    cpu = str(p.get('monit', {}).get('cpu', 0)) + '%'
    mem = str(round(p.get('monit', {}).get('memory', 0) / 1024 / 1024, 1)) + 'MB'
    uptime = p.get('pm2_env', {}).get('pm_uptime', 0)
    if uptime:
        import time
        secs = int(time.time() * 1000 - uptime) // 1000
        h, m = secs // 3600, (secs % 3600) // 60
        uptime_str = f'{h}h {m}m'
    else:
        uptime_str = 'N/A'
    status_icon = '🟢' if status == 'online' else '🔴'
    print(f'{p.get(\"pm_id\", 0):<4} {name:<22} {status_icon} {status:<7} {cpu:<6} {mem:<10} {uptime_str}')
"`
	out, err := sshClient.Run(cmd)
	if err != nil {
		out, err = sshClient.Run("pm2 ls")
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// NginxReload recarga la configuracion de nginx via SSH
func NginxReload(sshClient *ssh.Client) (string, error) {
	out, err := sshClient.Run("sudo nginx -s reload")
	if err != nil {
		return out, err
	}
	return "Nginx recargado correctamente", nil
}

// NginxTest verifica la configuracion de nginx via SSH
func NginxTest(sshClient *ssh.Client) (string, error) {
	out, err := sshClient.Run("sudo nginx -t")
	if err != nil {
		return out, err
	}
	return out, nil
}

// GetNginxConfig obtiene la lista de configuraciones de nginx
func GetNginxConfig(sshClient *ssh.Client) (string, error) {
	cmd := `echo '══════════════════════════════════════════'
echo '         NGINX - SITES HABILITADOS'
echo '══════════════════════════════════════════'
echo ''
for f in /etc/nginx/sites-enabled/*; do
    if [ -f "$f" ]; then
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📄 $(basename $f)"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        cat "$f"
        echo ''
    fi
done`
	out, err := sshClient.Run(cmd)
	if err != nil {
		return out, err
	}
	return out, nil
}

// NginxCopyConfig obtiene la configuracion de nginx y la copia al clipboard local
func NginxCopyConfig(sshClient *ssh.Client) (string, error) {
	cmd := `for f in /etc/nginx/sites-enabled/*; do
    if [ -f "$f" ]; then
        echo "# -- $(basename $f) --"
        echo ""
        cat "$f"
        echo ""
        echo ""
    fi
done`

	configOutput, err := sshClient.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("error obteniendo config: %v", err)
	}

	clipCmd := exec.Command("clip")
	clipCmd.Stdin = strings.NewReader(configOutput)
	err = clipCmd.Run()
	if err != nil {
		return "", fmt.Errorf("error copiando al clipboard: %v", err)
	}

	lines := len(strings.Split(configOutput, "\n"))
	return fmt.Sprintf("Configuracion copiada al clipboard\n\n%d lineas copiadas\n\nPuedes pegar (Ctrl+V) en cualquier editor", lines), nil
}
