# TODO

## Servidor
- [ ] Monitor de recursos - CPU, RAM, disco en tiempo real (top, free, df)
- [ ] Gestión de archivos .env - Ver, editar y comparar variables de entorno
- [ ] Terminal SSH interactiva - Mini shell para comandos ad-hoc

## Deploy
- [ ] Confirmación pre-deploy - Mostrar git log de cambios antes de ejecutar
- [ ] Deploy selectivo - Elegir pasos: solo pull, build, restart, etc.
- [ ] Historial de deploys - Log local con fecha, proyecto y resultado

## PM2
- [ ] Reload graceful suelto (`r`) - hoy solo corre dentro del deploy, y solo aporta en cluster
- [ ] Stop/Delete procesos
- [ ] Escalar instancias (pm2 scale)
- [ ] Métricas en tiempo real - CPU/RAM por proceso

## Nginx
- [ ] Editor de config inline
- [ ] Habilitar/deshabilitar sites (symlinks)
- [ ] Streaming de access/error logs

## UX
- [ ] Soporte multi-servidor con selector
- [ ] Notificaciones Discord/Slack via webhook
- [ ] Temas de colores (Catppuccin Latte, Dracula, etc.)
- [x] Atajos rápidos con teclas numéricas (1-5, tabs)
- [ ] Búsqueda de logs estilo vim (/)

## Seguridad
- [ ] Health check HTTP post-deploy
