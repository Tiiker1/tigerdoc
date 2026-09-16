# Update & uninstall

## Update

The one-line installer always fetches the latest `main` build. For an installed server, use the updater, which detects your current setup (port, allowed subnets, stop timeout) and reapplies it:

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/update.sh | sudo sh
```

Flags passed to `update.sh` are forwarded to the installer and override the detected settings:

```sh
# keep current port, change subnets
curl -fsSL .../update.sh | sudo sh -s -- --subnets 10.0.0.0/8
```

The build timestamp shown in the page footer confirms which build is running.

## Uninstall

```sh
curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/uninstall.sh | sudo sh
```

What it does:

1. Stops and disables the `docker-dashboard` systemd service.
2. Removes `/etc/systemd/system/docker-dashboard.service` and reloads systemd.
3. Kills any remaining process.
4. Deletes the binary (from `/usr/local/bin`, or wherever `--dir` placed it).

**Containers are never touched** — uninstalling the dashboard only removes the dashboard itself.