# Conventions

## Comentarios

Prioridad siempre: código autodocumentado. Nombres claros, funciones chicas,
estructura obvia > comentarios. Un comentario es la excepción, no la norma.

Comenta solo lo que el código no puede decir por sí mismo — el **por qué**,
no el qué. Si borrar el comentario no le quita nada a quien lee el código,
sobra.

No:

- Comentario que repite el nombre de la función/variable.
- Encabezados decorativos o separadores tipo banner.
- Un comentario por línea o por campo "explicando" lo obvio.

### Forma

- **Una línea.** Si no entra, casi siempre sobra texto.
- **Excepción:** los que explican un comando destructivo en el servidor (`rm -rf`,
  `mv`, el swap de renames). Ahí el porqué completo vale más que la brevedad.
- **Todo comentario lleva tag**, salvo dos casos: los doc comments de Go que
  empiezan con el nombre de la función y los rótulos de un bloque de datos.
- **Al final de línea solo para glosar un valor**, nunca para un `WHY:`:

```go
inner := width - 6 // 2 de borde + 4 de padding
```

### Tags (Better Comments)

Cuando un comentario es necesario, usa estos tags — coinciden con
`.vscode/settings.json`:

| Tag         | Uso                                                           |
| ----------- | ------------------------------------------------------------- |
| `WHY:`      | Razón detrás de una decisión no obvia                         |
| `SYNC:`     | Invariante entre goroutines, canal o mutex                    |
| `NOTE:`     | Contexto externo que no se deduce del código (SSH, PM2, YAML) |
| `SECURITY:` | Config que se interpola sin comillas en un comando remoto     |

```go
// WHY: sin mutex — la UI es secuencial, no hay escrituras concurrentes.
```

### Estilo

- `gofmt -l .` sin marcas antes de commitear.
- Nombres de función/variable en inglés; comentarios, mensajes de error y
  texto para el usuario final, en español (consistente con el resto del código).
- Preferir menos líneas y menos archivos: si una función o archivo crece
  mezclando responsabilidades distintas, se separa; si no, se deja junto.

## Commits

[Conventional Commits](https://www.conventionalcommits.org/), in English, kept short.

```
type: subject
```

| Type       | When                       |
| ---------- | -------------------------- |
| `feat`     | New user-facing behavior   |
| `fix`      | Fixes broken behavior      |
| `refactor` | Same behavior, better code |
| `docs`     | Only docs (`.md`)          |
| `chore`    | Deps, config, tooling      |

- Imperative, lowercase, no trailing period: `add restart-all`, not `added`.
- One commit = one change.
- Body only if the _why_ isn't obvious from the subject.
