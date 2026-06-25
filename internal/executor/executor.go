package executor

import (
	"fmt"
	"os/exec"
	"strings"

	"sdt/internal/config"
	"sdt/internal/ssh"
)

type StepResult struct {
	Step    string
	Success bool
	Output  string
	Error   string
}

// StepProgress se emite por el canal de Deploy para que la UI muestre, en vivo,
// en que paso va el deploy. Done/Failed indican el resultado del paso.
type StepProgress struct {
	Name   string
	Done   bool
	Failed bool
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

func (e *Executor) Deploy(progress chan<- StepProgress) error {
	steps := []struct {
		name string
		fn   func() (string, error)
	}{
		{"Actualizar codigo", e.gitPull},
		{"Instalar dependencias", e.installDeps},
	}

	// Tests opcionales: solo si el proyecto define test_cmd. Si fallan, el deploy
	// se aborta ANTES del build (no se toca nada en el servidor).
	if strings.TrimSpace(e.project.TestCmd) != "" {
		steps = append(steps, struct {
			name string
			fn   func() (string, error)
		}{"Ejecutar tests", e.runTests})
	}

	steps = append(steps, struct {
		name string
		fn   func() (string, error)
	}{"Build", e.build})

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
		progress <- StepProgress{Name: step.name} // running

		output, err := step.fn()
		if err != nil {
			e.results = append(e.results, StepResult{
				Step:    step.name,
				Success: false,
				Output:  output,
				Error:   err.Error(),
			})
			progress <- StepProgress{Name: step.name, Failed: true}
			return err
		}

		e.results = append(e.results, StepResult{
			Step:    step.name,
			Success: true,
			Output:  output,
		})
		progress <- StepProgress{Name: step.name, Done: true}
	}

	return nil
}

// gitPull hace un "pull especial": en vez de `git pull` (que puede fallar por
// conflictos si el servidor tiene cambios locales), trae el remoto y fuerza el
// estado del working tree a coincidir EXACTAMENTE con origin/<branch>.
func (e *Executor) gitPull() (string, error) {
	cmd := fmt.Sprintf(
		"cd %s && git fetch origin %s && git checkout %s && git reset --hard origin/%s",
		e.project.Path, e.project.Branch, e.project.Branch, e.project.Branch,
	)
	out, err := e.sshClient.Run(cmd)
	if err != nil {
		return out, fmt.Errorf("git update: %v", err)
	}

	return out, nil
}

// runTests ejecuta la suite de tests del proyecto. Solo se invoca cuando
// test_cmd esta definido (ver Deploy). Un fallo aborta el deploy.
func (e *Executor) runTests() (string, error) {
	cmd := fmt.Sprintf("cd %s && %s", e.project.Path, e.project.TestCmd)
	out, err := e.sshClient.Run(cmd)
	if err != nil {
		return out, err
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

// build construye el proyecto con respaldo y rollback automaticos.
//
// Si output_dir esta definido: respalda el build anterior, construye, y si el
// build falla restaura el respaldo (el sitio sigue sirviendo la version previa).
// Si el build tiene exito, borra el respaldo. Replica el deploy.yml del CI.
//
// Si output_dir no esta definido, simplemente construye (sin red de seguridad).
func (e *Executor) build() (string, error) {
	path := e.project.Path
	dir := strings.TrimSpace(e.project.OutputDir)
	withBackup := dir != ""

	if withBackup {
		// Respaldar build anterior. `|| true` para no fallar si no existe aun.
		backup := fmt.Sprintf(
			"cd %s && rm -rf %s.backup && { cp -r %s %s.backup 2>/dev/null || true; }",
			path, dir, dir, dir,
		)
		if out, err := e.sshClient.Run(backup); err != nil {
			return out, fmt.Errorf("respaldo: %v", err)
		}
	}

	out, err := e.sshClient.Run(fmt.Sprintf("cd %s && %s", path, e.project.BuildCmd))
	if err != nil {
		if withBackup {
			// Build fallido: restaurar el build anterior.
			restore := fmt.Sprintf(
				"cd %s && rm -rf %s && { mv %s.backup %s 2>/dev/null || true; }",
				path, dir, dir, dir,
			)
			e.sshClient.Run(restore) // best-effort: ya estamos en el camino de error
			return out, fmt.Errorf("build fallido, se restauro el build anterior: %v", err)
		}
		return out, err
	}

	if withBackup {
		// Build OK: descartar el respaldo.
		e.sshClient.Run(fmt.Sprintf("cd %s && rm -rf %s.backup", path, dir))
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
