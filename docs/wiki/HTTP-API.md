# HTTP API

All routes are behind the [LAN guard](Security). Errors are returned as JSON: `{"error": "message"}`.

## Endpoints

| Method | Path                          | Description |
| ------ | ----------------------------- | ----------- |
| GET    | `/api/system`                 | Engine version, host, container counts |
| GET    | `/api/build`                  | Binary version + build timestamp |
| GET    | `/api/containers`             | All containers (all states) |
| POST   | `/api/containers/{id}/start`  | Start a container |
| POST   | `/api/containers/{id}/stop`   | Stop (graceful, `STOP_TIMEOUT`) |
| POST   | `/api/containers/{id}/restart`| Restart a container |
| POST   | `/api/containers/{id}/pause`  | Pause a container |
| POST   | `/api/containers/{id}/unpause`| Unpause a container |
| POST   | `/api/containers/{id}/remove` | Remove a container (`?force=true` allowed) |
| GET    | `/ws/logs`                    | WebSocket log stream |

## Example

```sh
curl http://server:8080/api/containers
curl -X POST http://server:8080/api/containers/abc123/stop
curl -X POST "http://server:8080/api/containers/abc123/remove?force=true"
```

## WebSocket logs

Query parameters:

| Param        | Values                | Description |
| ------------ | --------------------- | ----------- |
| `id`         | container ID          | Required |
| `tail`       | `100` / `500` / `2000` / `all` | Lines to read first |
| `follow`     | `1`                   | Stream new output as it appears |
| `timestamps` | `1`                   | Prefix each line with a timestamp |

```sh
wscat -c "ws://server:8080/ws/logs?id=abc123&tail=100&follow=1&timestamps=1"
```

The stream is demultiplexed from Docker's stdout/stderr framing; the connection closes cleanly when the container stops and `follow` is off.