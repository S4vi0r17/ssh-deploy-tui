package executor

import (
	"fmt"
	"os/exec"
	"strings"

	"sdt/internal/config"
	"sdt/internal/ssh"
)

// WHY: un hueco en build_cmd sirve para cualquier toolchain sin hardcodear su flag.
const outPlaceholder = "{{out}}"

type StepResult struct {
	Step    string
	Success bool
	Output  string
	Error   string
}

// WHY: Done/Failed como dos bools y no un enum: el valor cero queda en "running".
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

	// WHY: los tests van antes del build, asi un fallo aborta sin tocar el servidor.
	if strings.TrimSpace(e.project.TestCmd) != "" {
		steps = append(steps, struct {
			name string
			fn   func() (string, error)
		}{"Ejecutar tests", e.runTests})
	}

	buildStep := "Build"
	if strings.TrimSpace(e.project.OutputDir) != "" && strings.Contains(e.project.BuildCmd, outPlaceholder) {
		buildStep = "Build + swap atomico"
	}
	steps = append(steps, struct {
		name string
		fn   func() (string, error)
	}{buildStep, e.build})

	if e.project.Type == "pm2" {
		steps = append(steps, struct {
			name string
			fn   func() (string, error)
		}{"Limpiar logs", e.flushPM2})
		steps = append(steps, struct {
			name string
			fn   func() (string, error)
		}{"Reload PM2 (zero-downtime)", e.reloadPM2})
	}

	for _, step := range steps {
		progress <- StepProgress{Name: step.name} // corriendo

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

// WHY: fetch + reset --hard y no `git pull`, que falla si el server tiene cambios locales.
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

func (e *Executor) build() (string, error) {
	path := e.project.Path
	dir := strings.TrimSpace(e.project.OutputDir)

	if dir == "" {
		return e.sshClient.Run(fmt.Sprintf("cd %s && %s", path, e.project.BuildCmd))
	}
	if !validOutputDir(dir) {
		return "", fmt.Errorf("output_dir invalido: %q (debe ser una ruta relativa dentro del proyecto, sin metacaracteres)", dir)
	}
	if strings.Contains(e.project.BuildCmd, outPlaceholder) {
		return e.buildStaged(path, dir)
	}
	return e.buildInPlace(path, dir)
}

func (e *Executor) buildStaged(path, dir string) (string, error) {
	staged, previous := dir+".new", dir+".old"

	clean := fmt.Sprintf("cd %s && rm -rf %s %s", path, staged, previous)
	if out, err := e.sshClient.Run(clean); err != nil {
		return out, fmt.Errorf("limpiar build anterior: %v", err)
	}

	cmd := strings.ReplaceAll(e.project.BuildCmd, outPlaceholder, staged)
	out, err := e.sshClient.Run(fmt.Sprintf("cd %s && %s", path, cmd))
	if err != nil {
		e.sshClient.Run(fmt.Sprintf("cd %s && rm -rf %s", path, staged))
		return out, fmt.Errorf("build fallido, %s sigue intacto: %v", dir, err)
	}

	// WHY: un build_cmd que ignora {{out}} sale con 0 sin escribir staged, y el
	// swap publicaria un directorio vacio sobre el sitio vivo.
	check := fmt.Sprintf(`cd %s && [ -d %s ] && [ -n "$(ls -A %s)" ]`, path, staged, staged)
	if _, err := e.sshClient.Run(check); err != nil {
		return out, fmt.Errorf("el build no genero %s, no se hizo swap (revisa que build_cmd use %s)", staged, outPlaceholder)
	}

	// WHY: rename(2) no intercambia dos directorios en una llamada, asi que la
	// ventana son los microsegundos entre ambos mv. `|| true` cubre el primer
	// deploy, donde todavia no hay dir vivo que mover.
	swap := fmt.Sprintf(
		"cd %s && { mv %s %s 2>/dev/null || true; } && mv %s %s",
		path, dir, previous, staged, dir,
	)
	if sout, err := e.sshClient.Run(swap); err != nil {
		e.sshClient.Run(fmt.Sprintf("cd %s && { [ -d %s ] || mv %s %s; }", path, dir, previous, dir))
		return sout, fmt.Errorf("swap fallido, se restauro el build anterior: %v", err)
	}

	e.sshClient.Run(fmt.Sprintf("cd %s && rm -rf %s", path, previous))

	return out, nil
}

// WHY: compila sobre el dir vivo y el sitio cae durante el build; con {{out}} hay swap.
func (e *Executor) buildInPlace(path, dir string) (string, error) {
	// WHY: `if` y no `[ -d x ] && cp ... || true`, que se traga un cp a medias
	// y deja un respaldo corrupto que un build fallido restaura.
	backup := fmt.Sprintf(
		"cd %s && rm -rf %s.backup && if [ -d %s ]; then cp -r %s %s.backup; fi",
		path, dir, dir, dir, dir,
	)
	if out, err := e.sshClient.Run(backup); err != nil {
		return out, fmt.Errorf("respaldo: %v", err)
	}

	out, err := e.sshClient.Run(fmt.Sprintf("cd %s && %s", path, e.project.BuildCmd))
	if err != nil {
		// WHY: mover el build roto en vez de borrarlo primero deja el dir vivo
		// ausente dos renames y no lo que tarde un rm -rf.
		restore := fmt.Sprintf(
			"cd %s && if [ -d %s.backup ]; then mv %s %s.broken 2>/dev/null || true; mv %s.backup %s; rm -rf %s.broken; fi",
			path, dir, dir, dir, dir, dir, dir,
		)
		e.sshClient.Run(restore)
		return out, fmt.Errorf("build fallido, se restauro el build anterior: %v", err)
	}

	e.sshClient.Run(fmt.Sprintf("cd %s && rm -rf %s.backup", path, dir))

	return out, nil
}

// SECURITY: dir se interpola sin comillas en rm -rf y mv.
func validOutputDir(dir string) bool {
	if strings.HasPrefix(dir, "/") || strings.HasPrefix(dir, "~") {
		return false
	}
	if strings.ContainsAny(dir, " \t\n;&|$`<>()*?[]{}\"'\\") {
		return false
	}
	for _, seg := range strings.Split(dir, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func (e *Executor) flushPM2() (string, error) {
	out, err := e.sshClient.Run(fmt.Sprintf("pm2 flush %s", e.project.PM2Name))
	if err != nil {
		return out, err
	}
	return out, nil
}

// WHY: `reload` cambia los workers sin downtime; cae a `restart` si el proceso no existe aun.
func (e *Executor) reloadPM2() (string, error) {
	cmd := fmt.Sprintf(
		"pm2 reload %s --update-env || pm2 restart %s --update-env",
		e.project.PM2Name, e.project.PM2Name,
	)
	out, err := e.sshClient.Run(cmd)
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

// NOTE: en cluster cada worker es una fila propia repitiendo el total de instancias.
func GetPM2Status(sshClient *ssh.Client) (string, error) {
	cmd := `pm2 jlist | python3 -c "
import sys, json, time
data = json.load(sys.stdin)
print('%-4s %-22s %-10s %-12s %-6s %-10s %s' % ('ID', 'NOMBRE', 'STATUS', 'MODO', 'CPU', 'MEM', 'UPTIME'))
print('-' * 82)
for p in data:
    env = p.get('pm2_env', {})
    name = p.get('name', '')[:20]
    status = env.get('status', 'N/A')
    mode = env.get('exec_mode', '').replace('_mode', '') or 'N/A'
    if mode == 'cluster':
        mode = 'cluster x' + str(env.get('instances') or 1)
    cpu = str(p.get('monit', {}).get('cpu', 0)) + '%'
    mem = str(round(p.get('monit', {}).get('memory', 0) / 1024 / 1024, 1)) + 'MB'
    uptime = env.get('pm_uptime', 0)
    if uptime:
        secs = int(time.time() * 1000 - uptime) // 1000
        h, m = secs // 3600, (secs % 3600) // 60
        uptime_str = f'{h}h {m}m'
    else:
        uptime_str = 'N/A'
    status_icon = '🟢' if status == 'online' else '🔴'
    print(f'{p.get(\"pm_id\", 0):<4} {name:<22} {status_icon} {status:<7} {mode:<12} {cpu:<6} {mem:<10} {uptime_str}')
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

func NginxReload(sshClient *ssh.Client) (string, error) {
	out, err := sshClient.Run("sudo nginx -s reload")
	if err != nil {
		return out, err
	}
	return "Nginx recargado correctamente", nil
}

func NginxTest(sshClient *ssh.Client) (string, error) {
	out, err := sshClient.Run("sudo nginx -t")
	if err != nil {
		return out, err
	}
	return out, nil
}

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
