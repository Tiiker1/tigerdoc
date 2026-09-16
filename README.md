# Docker Dashboard

A small, interactive Docker dashboard as a **single Linux binary**. It talks
directly to the Docker daemon over the socket and serves a web UI — no
container, database, or CDN assets. **LAN-only by default**: requests from
non-private source IPs get `403`.

Full documentation lives in [`docs/wiki/`](docs/wiki) and is auto-published to
the GitHub wiki on every push.

## Features

- Live container list with search, state filter, and auto-refresh
- Start / stop / restart / pause / unpause / remove containers
- Live streaming logs over WebSocket (follow, tail, timestamps)
- Dark / light theme, responsive

## Quick start

On the Linux server that runs Docker:

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh | sudo sh
```

Downloads the binary, verifies its checksum, installs it to
`/usr/local/bin/docker-dashboard`, and enables a systemd service. Then open
`http://<server-ip>:8080`.

No releases or versions needed — binaries are rebuilt on every push to `main`.

| Install flags       | Description                                  |
| ------------------- | -------------------------------------------- |
| `--port <n>`        | Port to listen on (default `8080`)           |
| `--subnets <cidrs>` | Comma-separated allowed CIDRs (default: LAN) |
| `--dir <path>`      | Install directory (default `/usr/local/bin`) |
| `--release`         | Install from the latest GitHub release       |
| `--version <tag>`   | Install a specific release tag               |
| `--no-systemd`      | Binary only; no service                      |

## Update & uninstall

```sh
# Update (keeps your current port/subnets/settings)
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/update.sh | sudo sh

# Uninstall completely
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/uninstall.sh | sudo sh
```

## Configuration

| Variable          | Default                       | Description |
| ----------------- | ----------------------------- | ----------- |
| `DASHBOARD_ADDR`  | `:8080`                       | HTTP bind address; e.g. `192.168.1.5:8080` binds one interface. |
| `DOCKER_HOST`     | `unix:///var/run/docker.sock` | Daemon endpoint (`unix://` or `tcp://`). |
| `ALLOWED_SUBNETS` | private ranges                | Comma-separated CIDRs; overrides the LAN allowlist. |
| `ALLOW_ALL`       | unset                         | `true` disables the source-IP guard (not recommended). |
| `STOP_TIMEOUT`    | `10`                          | Seconds to wait on stop/restart before force-kill. |

Edit settings on the server with `sudo systemctl edit docker-dashboard`.

## Security

The default allowlist is `10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16,
169.254.0.0/16, 127.0.0.0/8, ::1/128, fc00::/7, fe80::/10`. Requests from
public IPs get `403`. **Do not port-forward the port**, and don't set
`ALLOW_ALL=true` on a host with a public address. There is no authentication
yet — the LAN guard is today's protection.

## Usage

- Table lists all containers; search filters by name/image/id, buttons filter by state.
- Per-row actions appear per state: **logs**, start, stop, restart, pause, unpause, remove.
- Log viewer: **follow** (live), **timestamps**, and a **tail** size.

## HTTP API

| Method | Path                          | Description |
| ------ | ----------------------------- | ----------- |
| GET    | `/api/system`                 | Engine version, host, counts |
| GET    | `/api/containers`             | All containers |
| POST   | `/api/containers/{id}/start`  | Start |
| POST   | `/api/containers/{id}/stop`   | Stop (graceful, `STOP_TIMEOUT`) |
| POST   | `/api/containers/{id}/restart`| Restart |
| POST   | `/api/containers/{id}/pause`  | Pause |
| POST   | `/api/containers/{id}/unpause`| Unpause |
| POST   | `/api/containers/{id}/remove` | Remove (`?force=true` allowed) |
| GET    | `/ws/logs?id=…&tail=500&follow=1&timestamps=1` | WebSocket log stream |

## Development

```sh
go build ./...  # build
go vet ./...    # static checks
go test ./...   # unit + integration tests (mock daemon)
```

Layout: `main.go` entrypoint · `config.go` config + subnet guard ·
`docker.go` Docker Engine API client · `server.go` routes/middleware ·
`websocket.go` log streaming · `static/` embedded UI · `install.sh`,
`update.sh`, `uninstall.sh` scripts · `deploy/docker-dashboard.service` unit.

## Releases (maintainers, optional)

Versioned releases work but aren't required: push a tag (`git tag v1.0.0 && git push origin v1.0.0`)
and GitHub Actions publishes binaries. Install one with `--release` or
`--version v1.0.0`.