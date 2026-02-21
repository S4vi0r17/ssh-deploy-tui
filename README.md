# SSH Deploy TUI

Terminal UI tool to deploy and manage projects on remote servers via SSH. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![SSH Deploy TUI](resources/preview.png)

## Features

- **Deploy projects** — git pull, install dependencies, build and restart in one step
- **PM2 management** — view status, stream real-time logs, restart services
- **Nginx management** — view/copy config, test syntax, reload
- **Multi-project support** — manage multiple projects (PM2 or static) from a single config
- **SSH connection** — password or key-based authentication

## Prerequisites

- [Go](https://go.dev/) 1.24+

## Setup

```bash
# Clone
git clone https://github.com/S4vi0r17/ssh-deploy-tui.git
cd ssh-deploy-tui

# Install dependencies
go mod tidy

# Configure
cp config.example.yaml config.yaml
# Edit config.yaml with your SSH credentials and projects

# Run directly
go run .

# Or build and run the binary
go build -o ssh-deploy-tui .
./ssh-deploy-tui
```

## Configuration

```yaml
app_name: 'My Deploy'

ssh:
  host: your-server-ip
  port: 22
  user: your-user
  password: 'your-password'
  # key_path: ~/.ssh/id_rsa

projects:
  my-backend:
    name: 'My Backend'
    path: /var/www/html/my-backend
    type: pm2
    branch: main
    package_manager: pnpm
    install_cmd: pnpm install
    build_cmd: pnpm build
    pm2_name: my-backend

  my-frontend:
    name: 'My Frontend'
    path: /var/www/html/my-frontend
    type: static
    branch: main
    package_manager: bun
    install_cmd: bun install
    build_cmd: bun run build
    output_dir: dist

nginx:
  config_path: /etc/nginx/nginx.conf
  sites_path: /etc/nginx/sites-available
```

## Tech Stack

- [Go](https://go.dev/)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling
- [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto/ssh) — SSH client
