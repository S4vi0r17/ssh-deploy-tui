# TODO

## Servidor
- [ ] Monitor de recursos - CPU, RAM, disco en tiempo real (top, free, df)
- [ ] Gestión de archivos .env - Ver, editar y comparar variables de entorno
- [ ] Terminal SSH interactiva - Mini shell para comandos ad-hoc

## Deploy
- [ ] Rollback - Volver al commit anterior si un deploy falla
- [ ] Confirmación pre-deploy - Mostrar git log de cambios antes de ejecutar
- [ ] Deploy selectivo - Elegir pasos: solo pull, build, restart, etc.
- [ ] Historial de deploys - Log local con fecha, proyecto y resultado

## PM2
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
- [ ] Atajos rápidos con teclas numéricas (1-6)
- [ ] Búsqueda de logs estilo vim (/)

## Seguridad
- [ ] Backup antes de deploy (snapshot del directorio)
- [ ] Health check HTTP post-deploy
