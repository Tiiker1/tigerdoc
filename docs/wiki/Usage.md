# Usage

Open `http://<server-ip>:8080` from a machine on your LAN.

## Container table

The table lists all containers, running first. Each row shows **state**, **name** (with short ID), **image**, **status**, **ports**, **created** time, and available actions.

- **Search** filters by name, image, or ID.
- **State filter** buttons narrow the list: `all`, `running`, `paused`, `stopped`.
- **auto−refresh** toggles 3-second list polling; **↻** refreshes immediately.

## Actions per state

| State             | Available actions |
| ----------------- | ----------------- |
| running           | logs, stop, restart, pause, remove |
| paused            | logs, unpause, stop, restart |
| exited / created / dead | logs, start, remove |
| restarting / removing   | none (transient) |

Removing a container shows a confirmation prompt first.

## Log viewer

Click **logs** on any row to open the viewer:

- **follow** — stream live output
- **timestamps** — prepend timestamps to each line
- **tail** — number of lines to load (100 / 500 / 2000 / all)
- Live stream reconnects automatically on disconnect; lines are line-buffered across WebSocket frames.

## Theme

Use the moon/sun button in the top bar to switch between **dark** and **light**. The choice is remembered per browser (localStorage).

## Adding containers

The dashboard currently **manages** existing containers — it does not create them. To add a container, create it on the server and it appears in the table automatically:

```sh
docker run -d --name myapp -p 8080:80 nginx
```

Automatic container creation from the UI is on the [roadmap](Planned).