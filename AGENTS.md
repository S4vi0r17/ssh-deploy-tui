# AGENTS.md

`sdt` — TUI en Bubble Tea que deploya y opera proyectos en un VPS por SSH:
pull, install, test, build, PM2, nginx, logs y tunnels.

## Comandos

```bash
go build ./...   # tiene que pasar
go vet ./...     # tiene que pasar
gofmt -l .       # no tiene que imprimir nada
```

No hay tests. La lógica de shell que va al servidor se prueba antes contra un
directorio de scratch local.

## Estructura

| Path                | Responsabilidad                                            |
| ------------------- | ---------------------------------------------------------- |
| `main.go`           | Carga config y arranca                                     |
| `internal/config`   | Schema YAML                                                |
| `internal/ssh`      | Conexión, `Run` y `RunStream`                              |
| `internal/executor` | Pasos de deploy, PM2, nginx. Todo el shell remoto vive acá |
| `internal/tunnel`   | Port forwarding local                                      |
| `internal/ui`       | Modelo Bubble Tea y render                                 |

## Antes de tocar el lado del servidor

- **Cada `Run` abre una sesión SSH nueva**: no persiste cwd ni entorno. Por eso
  cada comando empieza con `cd <path> &&` y lleva `init_cmd` prefijado.
- **Un paso es un solo `Run`.** Si dos comandos tienen que pasar juntos, van
  encadenados con `&&` en la misma llamada: una desconexión entre dos `Run`
  deja el servidor a medio deployar.
- **La config se interpola sin comillas.** `output_dir` llega a `rm -rf` y `mv`,
  por eso pasa por `validOutputDir`. Un campo nuevo que llegue a un comando
  necesita lo mismo.
- **El deploy no puede tirar el sitio.** Con `output_dir` y `{{out}}` el build
  va a `<output_dir>.new` y se publica con un swap de renames.

## UI

Loop estándar de Bubble Tea. Toda llamada SSH vive en un `tea.Cmd`, nunca en
`Update`. El deploy corre en goroutine y reporta pasos por un canal que drena
`waitForDeploy`.

## Convenciones

`CONVENTIONS.md` manda para comentarios y commits — leerlo antes de escribir
cualquiera de los dos.

Identificadores en inglés; errores y texto de pantalla en español.

## No

- Commitear `config.yaml` (tiene credenciales). Un campo nuevo va a
  `config.example.yaml`, que es la documentación de la config.
- Agregar dependencias que cubra la stdlib.
