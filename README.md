# Docker Dashboard

A small, interactive Docker dashboard that runs as a **single Linux binary**.
No container, no external services, no database, no CDN assets — it talks
directly to the Docker daemon on the host and serves a web UI to your browser.

It is **LAN-only by default**: every request whose source address is not in a
private/loopback range is rejected, so it cannot be reached from the internet
even if the host has a public IP.

## Features

- Live container list (state, image, ports, uptime) with auto-refresh
- Start / stop / restart / pause / unpause / remove containers
- Live streaming logs over WebSocket (follow / tail / timestamps)
- Search + state filter, dark theme, responsive

## Install (one line)

On the Linux server that runs Docker:

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh | sudo sh
```

That detects your architecture, downloads the binary from the repo's `main`
branch, verifies its checksum, installs it to `/usr/local/bin/docker-dashboard`,
and enables a systemd service. Then open `http://<server-ip>:8080` from a
machine on your LAN.

**No releases or versions needed** — GitHub Actions rebuilds the binaries on
every push to `main`, so the installer always gets a current build. Installing
a specific release is optional (`--release` or `--version v1.0.0`).

Customize with flags:

```sh
# different port, restrict to one subnet
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh \
  | sudo sh -s -- --port 9000 --subnets 192.168.1.0/24
```

### Installer options

| Flag                | Env var           | Default          | Description |
| ------------------- | ----------------- | ---------------- | ----------- |
| `--release`         | `SOURCE=release`  | branch `main`    | Install from the latest GitHub release |
| `--version <tag>`   | `VERSION`         | —                | Install a specific release tag, e.g. `v1.0.0` |
| `--port <n>`        | `PORT`            | `8080`           | Port the dashboard listens on |
| `--subnets <cidrs>` | `ALLOWED_SUBNETS` | private ranges   | Comma-separated CIDRs allowed to connect |
| `--dir <path>`      | `INSTALL_DIR`     | `/usr/local/bin` | Where to install the binary |
| `--no-systemd`      | `WITH_SYSTEMD=no` | auto             | Install the binary only; no service |

## Manual build (alternative)

From any machine with Go 1.23+:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o docker-dashboard .
scp docker-dashboard user@server:/tmp/
```

Then on the server: `sudo install -m 0755 /tmp/docker-dashboard /usr/local/bin/`.

For ARM servers (Raspberry Pi, ARM cloud hosts) use `GOARCH=arm64` or
`GOARCH=arm GOARM=7`.

## Service management

```sh
sudo systemctl status docker-dashboard
sudo systemctl restart docker-dashboard
sudo journalctl -u docker-dashboard -f
```

Editing settings: `sudo systemctl edit docker-dashboard` and add, for example:

```ini
[Service]
Environment=ALLOWED_SUBNETS=10.0.0.0/8
Environment=STOP_TIMEOUT=15
```

then `sudo systemctl restart docker-dashboard`.

## Configuration (environment variables)

| Variable          | Default                       | Description |
| ----------------- | ----------------------------- | ----------- |
| `DASHBOARD_ADDR`  | `:8080`                       | HTTP bind address. Use `192.168.1.5:8080` to bind one LAN interface. |
| `DOCKER_HOST`     | `unix:///var/run/docker.sock` | Docker daemon endpoint (`unix://...` or `tcp://...`). |
| `ALLOWED_SUBNETS` | private ranges                | Comma-separated CIDRs; overrides the LAN allowlist. |
| `ALLOW_ALL`       | unset                         | Set to `true` to disable the source-IP guard (not recommended). |
| `STOP_TIMEOUT`    | `10`                          | Seconds to wait after SIGTERM before SIGKILL on stop/restart. |

## The LAN guard

Default allowlist (all private + link-local ranges):

`10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, 127.0.0.0/8, ::1/128, fc00::/7, fe80::/10`

Requests from public IPs get `403`. Keep it that way: **do not port-forward the
port**, and never set `ALLOW_ALL=true` on a machine with a public address.

## Usage

- The table lists all containers, running first. Search filters by name/image/id;
  the `running / paused / stopped` buttons filter by state.
- Per-row actions: **logs**, start/stop/restart/pause/unpause/remove — shown
  according to the container's current state.
- Log viewer: toggle **follow** (live), **timestamps**, and choose a **tail** size.
- `auto−refresh` toggles 3-second list polling; `↻` refreshes immediately.

## HTTP API

| Method | Path                          | Description |
| ------ | ----------------------------- | ----------- |
| GET    | `/api/system`                 | Engine version, host, counts |
| GET    | `/api/containers`             | All containers (all states) |
| POST   | `/api/containers/{id}/start`  | Start |
| POST   | `/api/containers/{id}/stop`   | Stop (graceful, `STOP_TIMEOUT`) |
| POST   | `/api/containers/{id}/restart`| Restart |
| POST   | `/api/containers/{id}/pause`  | Pause |
| POST   | `/api/containers/{id}/unpause`| Unpause |
| POST   | `/api/containers/{id}/remove` | Remove (`?force=true` allowed) |
| GET    | `/ws/logs?id=…&tail=500&follow=1&timestamps=1` | WebSocket log stream |

## Security notes

- The binary drives the Docker socket, so it can control every container on the
  host. Run it as root or as a user in the host's `docker` group.
- There is **no authentication yet** — the LAN guard is your protection today.
  Add auth before exposing it beyond a trusted network.

## Development

```sh
go build ./...     # build
go vet ./...       # static checks
go test ./...      # unit + integration tests (against a mock Docker daemon)
```

Layout: `main.go` entrypoint · `config.go` env config + subnet guard ·
`docker.go` minimal Docker Engine API client (stdlib HTTP + unix socket) ·
`server.go` routes/middleware · `websocket.go` live log streaming ·
`static/` embedded web UI · `install.sh` installer ·
`deploy/docker-dashboard.service` reference systemd unit.

## Releasing (maintainers, optional)

The one-line installer does not need releases. But if you want versioned
releases, push a tag — GitHub Actions builds linux `amd64`/`arm64`/`armv7`
binaries, generates `checksums.txt`, and publishes them as release assets:

```sh
git tag v1.0.0
git push origin v1.0.0
```

Install a release with `curl .../install.sh | sudo sh -s -- --release`
(or pin a tag with `--version v1.0.0`).

## Updates

If `dist/` is committed (the automatic build workflow does that for you), the
one-line installer always fetches the latest `main` build. To update an
installed server, just re-run the same installer command. You can also install
the reference systemd unit manually from `deploy/docker-dashboard.service`.
