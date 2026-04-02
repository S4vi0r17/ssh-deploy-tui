# SSH Deploy TUI

Deploy and manage remote projects from your terminal. Built with Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![SSH Deploy TUI](resources/preview.png)

## Features

- **One-step deploy** — git pull, install, build and restart
- **PM2** — status, real-time logs, restart
- **Nginx** — view config, test syntax, reload
- **Multi-project** — PM2 or static, all from one config

## Quick Start

```bash
git clone https://github.com/S4vi0r17/ssh-deploy-tui.git
cd ssh-deploy-tui
go mod tidy
cp config.example.yaml config.yaml  # edit with your data
go run .
```

> Requires [Go](https://go.dev/) 1.24+

## Configuration

```yaml
app_name: 'My Deploy'

ssh:
  host: your-server-ip
  port: 22
  user: your-user
  identity_file: ~/.ssh/id_rsa
  # Runs before every SSH command (non-interactive sessions don't load .bashrc)
  init_cmd: "source ~/.nvm/nvm.sh && export PATH=$HOME/.bun/bin:$HOME/.local/share/pnpm:$PATH && export GIT_SSH_COMMAND='ssh -i ~/.ssh/github_deploy_key -o IdentitiesOnly=yes'"

projects:
  my-backend:
    name: 'My Backend'
    path: /var/www/html/my-backend
    type: pm2 # managed by PM2
    branch: main
    package_manager: pnpm
    install_cmd: pnpm install
    build_cmd: pnpm build
    pm2_name: my-backend

  my-frontend:
    name: 'My Frontend'
    path: /var/www/html/my-frontend
    type: static # static site (no PM2)
    branch: main
    package_manager: bun
    install_cmd: bun install
    build_cmd: bun run build
    output_dir: dist

nginx:
  config_path: /etc/nginx/nginx.conf
  sites_path: /etc/nginx/sites-available
```

### `init_cmd`

Non-interactive SSH sessions don't load `.bashrc`, so `node`, `bun`, `pnpm`, etc. won't be found. `init_cmd` runs before every command to fix that.

| Part                                  | Why                                |
| ------------------------------------- | ---------------------------------- |
| `source ~/.nvm/nvm.sh`                | Load `node`/`npm`                  |
| `export PATH=...bun...pnpm...`        | Load `bun`/`pnpm`                  |
| `export GIT_SSH_COMMAND='ssh -i ...'` | Use a deploy key for private repos |

Adapt to your setup. If you only use `npm`:

```yaml
init_cmd: 'source ~/.nvm/nvm.sh'
```

## Tech Stack

[Go](https://go.dev/) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) · [Lip Gloss](https://github.com/charmbracelet/lipgloss) · [x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)
