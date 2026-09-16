# Troubleshooting

## Can't reach the dashboard from another device

- Check the dashboard is running: `sudo systemctl status docker-dashboard`.
- Check the port is listening: `ss -tlnp | grep 8080`.
- The device must be on an allowed subnet — see the [LAN guard](Configuration#the-lan-guard).
- Some firewalls block ports other than 80/443. Open 8080 (LAN-only) or change the port.

## I get `403 Forbidden`

Your client IP is not in the allowed subnets. Fix by narrowing/overriding with `ALLOWED_SUBNETS` (see [Configuration](Configuration)), or add a rule for your network. A `403` from a **public** IP is expected behaviour — do not loosen it.

## Installer says "no binaries found on the ... branch yet"

`dist/` is rebuilt by GitHub Actions **after** you push. If you ran the installer within the first minute or two of a push, wait and re-run:

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh | sudo sh
```

## Theme or UI looks old after an update

The binary serves its own embedded UI with `Cache-Control: no-cache`, so a rebuild is picked up on reload. If you still see a stale page, hard-reload (`Ctrl+Shift+R`). The build timestamp in the footer tells you which build is running.

## Logs keep reconnecting

The viewer reconnects automatically with backoff. Continuous reconnects usually mean the monitored container is restarting or the WebSocket closed for a different reason — check the container status in the table.

## "conflict" when removing

Removing a running container without `?force=true` is rejected. The UI prompts before removing; use the `force` query parameter for API callers.

## More help

Open an issue at <https://github.com/Tiiker1/tigerdoc/issues>.