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

Sí (cuando aplica):
- Por qué se eligió una solución no evidente sobre otra más simple.
- Una limitación externa (API, SO, librería) que fuerza el código a verse así.
- Un caso borde que no es obvio con solo leer la lógica.

## Tags (Better Comments)

Cuando un comentario es necesario, usa estos tags — coinciden con la config
de Better Comments en `.vscode/settings.json`:

| Tag          | Uso                                                        |
|--------------|-------------------------------------------------------------|
| `WHY:`       | Razón detrás de una decisión no obvia                       |
| `NOTE:`      | Contexto relevante que no es advertencia ni pendiente        |
| `TODO:`      | Trabajo pendiente                                            |
| `FIXME:`     | Bug conocido, falta arreglar                                 |
| `HACK:`      | Solución temporal/sucia, se haría distinto con más tiempo    |
| `PERF:`      | Código sensible a rendimiento, explica el trade-off          |
| `SECURITY:`  | Código relevante para seguridad (auth, validación, secretos) |
| `!`          | Crítico, debe leerse antes de tocar esa zona                 |
| `-`          | Deprecado / candidato a borrar                                |

```go
// WHY: gossh.Client no expone un evento de desconexión; SendRequest con
// timeout es la única forma de detectar un socket muerto tras un suspend.
func (c *Client) IsAlive() bool { ... }
```

## Estilo general

- `gofmt` sin overrides. Si `gofmt -l .` marca algo, se corrige antes de commitear.
- Preferir menos líneas y menos archivos: si una función o archivo crece
  mezclando responsabilidades distintas, se separa; si no, se deja junto.
- Nombres de función/variable en inglés; mensajes de error y texto para el
  usuario final, en español (consistente con el resto del código).
