# Security

## LAN-only by default

Every request's source IP is checked against an allowlist. The defaults are all private and link-local ranges:

```
10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16,
127.0.0.0/8, ::1/128, fc00::/7, fe80::/10
```

Requests from public IPs receive `403 Forbidden`, so even if the port were exposed, the dashboard could not be reached from the internet.

## Rules of thumb

- **Do not port-forward the dashboard port** on your router or firewall.
- **Never set `ALLOW_ALL=true`** on a host with a public address.
- **Restrict with `ALLOWED_SUBNETS`** when you want to limit access to one specific network:

  ```sh
  sudo systemctl edit docker-dashboard   # then:
  # Environment=ALLOWED_SUBNETS=192.168.1.0/24
  ```

## No authentication

The dashboard has **no authentication yet** — the LAN guard is today's only protection. Do not expose it beyond a trusted network. Any client on an allowed subnet can list, start, stop, and remove containers.

## Privilege model

- The binary drives the Docker socket, so it can control every container on the host.
- Run it **as root** or as a user in the host's **`docker`** group.
- The systemd unit ships with hardening options (`NoNewPrivileges`, `ProtectSystem=full`, `ProtectHome=true`, `PrivateTmp=true`).