> [Lire en français](container.fr.md)

# Running a session in a container

With opencode V2, a workflow that allows it (`runtime.allowed` contains `container`, for instance `ticket`) can run in a **container** built from the project's development Dockerfile. The agent's shell commands then run inside the container. Your secrets, Beads and the oh MCP servers stay on the machine.

```bash
oh run ticket -t bd-42 --runtime container
```

## Requirements

- macOS or Linux (Windows: local only).
- A container engine: **Colima** (`docker` runtime), **Podman** (machine started) or the **Docker** CLI. oh uses the engine CLI, not the Docker API.
- The project and its worktrees must be in a folder shared with the VM: under `$HOME` for Colima. `$TMPDIR` (`/var/folders/…`) is not shared with Colima.

## Configuring the project

TUI: **Project config › Execution**. The settings are stored in the oh database. oh writes nothing in the repository.

| Setting | Role | Default |
|---|---|---|
| Dev Dockerfile | Base image of the container. Path relative to the project or absolute. | Detected: `Dockerfile.dev`, `dev.Dockerfile`, `.devcontainer/Dockerfile`, `Dockerfile`. Without a file: oh image (`debian:bookworm-slim` + `git`, `ca-certificates`, `ripgrep`). |
| Build args | Build arguments: `KEY=value, KEY2=value` | none |
| Cache volumes | Persistent volumes, comma separated. A relative path applies to each mounted location (`node_modules`). An absolute path is a container path (`/root/.cache`). | none |
| Default workflow | Launched by `oh run` without argument. Shown first in « Start » (◆) and in the board actions. | none |
| Default runtime | Preferred runtime of the project, used when the workflow allows it | Settings, then the workflow default |

The Dockerfile is always looked up in the **project folder**, even when the session runs in a worktree. This also holds when a sleeping session resumes: the resume reads the project config again. If it changed, the image is rebuilt. The session data is kept.

### Runtime choice

From highest to lowest priority:

1. `--runtime` or the choice made in the launch form;
2. the project default runtime;
3. the default runtime of the Settings;
4. the workflow `runtime.default`.

A runtime the workflow does not allow (`runtime.allowed`) is skipped.

### In the launch form

The **▣ container** option shows whether the engine is available. When the VM is stopped or the pinned opencode version does not match, the option is marked ✗ with the reason and the launch is refused. When the option is selected, a line gives the state:

- `colima 28.1 · mount=virtiofs ✔ · image oh-dev/<project>:9f3c… cached`: nothing to build;
- `oh layer to rebuild (~40 s)`: only the oh layer changes (for instance after an opencode upgrade);
- `image to build: project environment + oh layer (~3 min)`: the Dockerfile or the build args changed.

The estimated time comes from the latest builds of the project. Before the first build, the line says « several minutes ». While launching, the last build lines are shown in the form.

### Git identity

The container `HOME` is a volume of the project's own: your `~/.gitconfig` is not there. So that the agent can commit, oh passes your git identity, read from the project, to the shell of the session (and of the subagents): `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, `GIT_COMMITTER_EMAIL`. No other git configuration is copied.

## Settings

TUI: **Settings › Execution**, or the `[execution]` section of `~/.oh/hub.toml`:

```toml
[execution]
runtime = "container"        # preferred runtime when the workflow allows it and the project sets none
engine = "colima"            # auto (default: Colima, then Podman, then Docker) | colima | podman | docker
keep_images = 2              # images kept per project and role (base, oh); the oldest are removed
opencode_version = "2.0.20"  # empty = the machine client version
strict_isolation = true      # also hides your opencode config from local servers
```

- **Pinned opencode version**: the image and the machine client must run the same version. When the pinned version differs from the client (for instance after a Homebrew upgrade), container launches are refused with the reason. Reinstall the pinned version or change the setting. Without a pinned version, the image follows the client: an opencode upgrade rebuilds the oh layer at the next launch.
- **Strict isolation**: locally, the server gets its own `XDG_CONFIG_HOME`. It is a copy of your config folder without `opencode/`, with links to the other folders, so git, gh… keep working. The plugins, agents and commands of your global opencode config are no longer loaded. In a container, this config is never visible.
- The engine choice replaces the `OH_CONTAINER_ENGINE` variable, which no longer exists.

## What happens at launch

1. **Image**: oh builds a base image from the dev Dockerfile (`oh-base/<project>:<hash>`), then a thin layer (`oh-dev/<project>:<hash>`). The layer adds opencode at the version of the machine client, and the fake `bd`. The tags depend on the content: an image already built is reused.
2. **Mounts**:
   - the project and the worktrees under `/work/<name>`, read-write;
   - the session bundle under `/opt/oh/bundle`, read-only;
   - the server group data under `/opt/oh/data`;
   - a `HOME` of the project's own: your opencode config is never visible.
3. **Network**: the server port is published on `127.0.0.1` only. The opencode client runs on the machine and attaches to the server in the container.
4. **Secrets**: no secret enters the container. The container gets an `ohs_…` token, which the oh credential proxy swaps for the real key. `bd` goes through the Beads gateway, with the workflow `beads.allow` list (read-only by default). The oh MCP servers (gitlab, team, workflow…) run on the machine.

## Checking with Doctor

`oh doctor` (or the TUI Doctor view) checks the container runtime and the gateways:

| Check | What is checked |
|---|---|
| Container: engine | Engine detected, version and details (`mount=virtiofs`, `rootless=true`). Without an installed engine, the check stays green: the container runtime is optional. |
| Container: pinned version | Pinned version different from the machine client |
| Container: file sharing | Colima: `virtiofs` mount advised (`sshfs` and `9p` are slow) |
| Container: user (keep-id) | Podman rootless: files created in the container belong to you |
| Container: folders shared with the VM | Projects and worktrees outside the shared folders: they would look empty |
| Container: image of `<project>` | `opencode --version` in the 3 latest images. On a musl base, reports missing `libstdc++` and `libgcc`. |
| Container: Linux listener | Linux: machine address seen from containers (second listener of the proxy and the gateways) |
| Beads gateway: bd on the machine | `bd` installed on the machine (otherwise the agent `bd` commands fail) |
| Gateways: oh daemon | Daemon recent enough to serve the gateways |
| Beads and MCP gateways: from a container | Request without token from a container to `/oh-gateway/…` and `/oh/v1/hooks/mcp/…`: a 401 proves the route is served on the address containers use |

The probes run a few short containers. They use the `busybox` image (downloaded the first time) or the latest project image when it has `curl` or `wget`.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| « container unavailable » in the launch form | VM stopped (`colima start`, `podman machine start`) or no engine |
| « … is not shared with the colima VM » | Project outside `$HOME` (Colima) |
| opencode does not start on an Alpine base | `libstdc++` and `libgcc` missing: add them to the Dockerfile |
