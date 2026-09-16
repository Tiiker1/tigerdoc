# Installation

## One line

On the Linux server that runs Docker:

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh | sudo sh
```

What it does:

1. Detects the architecture (`amd64`, `arm64`, or `armv7`).
2. Downloads the matching binary from the `main` branch.
3. Verifies its SHA-256 checksum against `dist/checksums.txt`.
4. Installs it to `/usr/local/bin/docker-dashboard`.
5. Enables a systemd service (`docker-dashboard.service`).

**No releases or versions are needed.** GitHub Actions rebuilds the binaries on every push to `main`, so the installer always fetches a current build.

## Options

Customize with flags:

```sh
# different port, restrict to one subnet
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh \
  | sudo sh -s -- --port 9000 --subnets 192.168.1.0/24
```

| Flag                | Env var           | Default          | Description |
| ------------------- | ----------------- | ---------------- | ----------- |
| `--release`         | `SOURCE=release`  | branch `main`    | Install from the latest GitHub release |
| `--version <tag>`   | `VERSION`         | —                | Install a specific release tag, e.g. `v1.0.0` |
| `--port <n>`        | `PORT`            | `8080`           | Port the dashboard listens on |
| `--subnets <cidrs>` | `ALLOWED_SUBNETS` | private ranges   | Comma-separated CIDRs allowed to connect |
| `--dir <path>`      | `INSTALL_DIR`     | `/usr/local/bin` | Where to install the binary |
| `--no-systemd`      | `WITH_SYSTEMD=no` | auto             | Install the binary only; no service |

## Running without systemd

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh \
  | sudo sh -s -- --no-systemd
docker-dashboard   # run it manually (as root or in the 'docker' group)
```

## Manual build (alternative)

From any machine with Go 1.23+:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o docker-dashboard .
scp docker-dashboard user@server:/tmp/
```

Then on the server:

```sh
sudo install -m 0755 /tmp/docker-dashboard /usr/local/bin/
```

For ARM servers use `GOARCH=arm64` or `GOARCH=arm GOARM=7`.