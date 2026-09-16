# Configuration

The dashboard is configured with environment variables. The installer writes them into the systemd unit, so on a standard install use `systemctl edit`:

```sh
sudo systemctl edit docker-dashboard
```

Add, for example:

```ini
[Service]
Environment=ALLOWED_SUBNETS=10.0.0.0/8
Environment=STOP_TIMEOUT=15
```

then restart:

```sh
sudo systemctl restart docker-dashboard
```

## Environment variables

| Variable          | Default                       | Description |
| ----------------- | ----------------------------- | ----------- |
| `DASHBOARD_ADDR`  | `:8080`                       | HTTP bind address. Use `192.168.1.5:8080` to bind one LAN interface. |
| `DOCKER_HOST`     | `unix:///var/run/docker.sock` | Daemon endpoint (`unix://` or `tcp://`). |
| `ALLOWED_SUBNETS` | private ranges                | Comma-separated CIDRs; overrides the LAN allowlist. |
| `ALLOW_ALL`       | unset                         | Set to `true` to disable the source-IP guard (not recommended). |
| `STOP_TIMEOUT`    | `10`                          | Seconds to wait after SIGTERM before SIGKILL on stop/restart. |

## The LAN guard

Default allowlist (all private + link-local ranges):

```
10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16,
127.0.0.0/8, ::1/128, fc00::/7, fe80::/10
```

Requests from public IPs get `403`. Keep it that way: **do not port-forward the port**, and never set `ALLOW_ALL=true` on a machine with a public address. See [Security](Security) for details.