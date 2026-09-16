# Development

## Commands

```sh
go build ./...     # build
go vet ./...       # static checks
go test ./...      # unit + integration tests
```

Tests run against a **mock Docker daemon** (`httptest`), so no Docker is needed on the dev machine.

## Layout

| File | Purpose |
| ---- | ------- |
| `main.go` | Entrypoint, `-version` flag, embeds `static/` |
| `config.go` | Env config + subnet guard |
| `docker.go` | Minimal Docker Engine API client (stdlib HTTP over unix socket) |
| `server.go` | Routes + LAN middleware, cache-aware static serving |
| `websocket.go` | Live log streaming + stdout/stderr demux |
| `server_test.go` | Tests against a mock daemon |
| `static/` | Embedded web UI (HTML, CSS, JS) |
| `install.sh`, `update.sh`, `uninstall.sh` | Server lifecycle scripts (POSIX sh) |
| `deploy/docker-dashboard.service` | Reference systemd unit |
| `docs/wiki/` | GitHub wiki pages (auto-published on push) |

## CI

- `build-dist.yml` — rebuilds `dist/` binaries on every push to `main` and commits them, so the installer always serves a current build.
- `release.yml` — publishes tagged releases (`v*`) with binaries + checksums.
- `wiki-sync.yml` — publishes `docs/wiki/*.md` to the GitHub wiki.

## Adding a Linux binary

The installer expects `dist/docker-dashboard-linux-{amd64,arm64,armv7}` plus `dist/checksums.txt`. `build-dist.yml` regenerates these automatically.