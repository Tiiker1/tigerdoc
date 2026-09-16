# Planned features

These are documented for reference only and are **not yet implemented**. Until they land, use the functionality described in [Usage](Usage).

## Create containers from the UI

Add the ability to create and run containers directly from the dashboard.

Planned design:

- **API**: `POST /api/containers` accepting
  `{image, name, cmd, env[], ports[], volumes[], restart, start}`.
- **Auto-pull**: if the image is not cached (`404` from the daemon), pull it first via `POST /images/create`, then retry create.
- **UI**: a **Create** button in the toolbar opens a modal form (image, name, command, env variables, port mappings, volumes, restart policy, "start now"). On success the list refreshes.
- **Errors**: map missing-image (404) and name/port conflict (409) to clear messages. New `PULL_TIMEOUT` config variable (default 600s).

Out of scope for v1: privileged/GPU containers, custom networks, exec-form commands — use the `docker` CLI for those.

_status: design only, no code yet._