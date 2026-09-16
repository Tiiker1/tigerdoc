# Docker Dashboard

A small, interactive Docker dashboard that runs as a **single Linux binary**. It talks directly to the Docker daemon over the socket and serves web UI to your browser — no container, no database, no CDN assets.

It is **LAN-only by default**: requests from non-private source IPs are rejected, so it cannot be reached from the internet even if the host has public IP.

## Features

- Live container list with search, state filter, and auto-refresh
- Start / stop / restart / pause / unpause / remove containers
- Live streaming logs over WebSocket (follow, tail, timestamps)
- Dark / light theme, responsive

## Quick install

On the Linux server that runs Docker:

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh | sudo sh
```

Then open `http://<server-ip>:8080` from a machine on your LAN.

No releases or versions needed — binaries are rebuilt on every push to `main`.

## Documentation

- [Installation](Installation)
- [Configuration](Configuration)
- [Usage](Usage)
- [Security](Security)
- [HTTP API](HTTP-API)
- [Update & uninstall](Update-Uninstall)
- [Development](Development)
- [Troubleshooting](Troubleshooting)
- [Planned features](Planned)
