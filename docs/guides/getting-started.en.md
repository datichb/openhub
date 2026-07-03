> 🇫🇷 [Lire en français](getting-started.fr.md)

# Quick Start

This guide gets you up and running with the hub and your first agent in under 10 minutes.

## Prerequisites

| Tool | Minimum version | Check |
|------|----------------|-------|
| Git | 2.x | `git --version` |
| curl | — | `curl --version` |

> Other dependencies (`jq`, `Node.js`, `opencode`, `bun`, `sqlite3`) are offered during installation — **each tool requires explicit confirmation** before being installed.
>
> **`sqlite3`** is required for `oh metrics` and `oh dashboard` (reads the OpenCode session database). It is **native on macOS** (`/usr/bin/sqlite3`); on Linux it will be offered via `apt-get` during install.
>
> **Beads (`bd`)** is offered during `oh install` (via `brew install beads` or curl).
> The terminal kanban board (`oh beads board`) is built-in — no additional installation required.

---

## 1. Install the hub

### Option A — One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

The script automates:
- Cloning the repo to `~/.openhub`
- Checking for missing dependencies (`jq`, `Node.js`, `opencode`, `bun`) — **confirmation requested before each installation**
- Creating the `oh` alias in `~/.zshrc` or `~/.bashrc` (offers to keep / replace / rename if an `oh` alias already exists)
- Initialising local config files
- Interactive configuration of the LLM provider

After installation, reload your shell:

```bash
source ~/.zshrc   # or source ~/.bashrc
```

> **Custom install directory:** `OPENCODE_HUB_DIR=~/tools/oc bash install.sh`

---

### Option B — Manual installation

```bash
# 1. Clone
git clone https://github.com/datichb/openhub.git ~/.openhub

# 2. Shell alias
echo 'alias oc="~/.openhub/oc.sh"' >> ~/.zshrc && source ~/.zshrc

# 3. Configure
oh install
```

`oh install` is interactive and configures the opencode integration:

| Choice | Target configured |
|--------|-------------------|
| 1 (default) | opencode |

> If `config/hub.json` already exists, confirmation is requested before overwriting
> the configuration. Answer `N` to keep your existing configuration.

---

## 2. Register a project

```bash
oh init MY-APP ~/workspace/my-app
```

This command:
- Adds `MY-APP` to `projects/projects.md`
- Associates the local path `~/workspace/my-app`
- Offers to deploy agents immediately

> **`PROJECT_ID` convention**: letters, digits, `-` and `_` only. No spaces.

---

## 3. Deploy agents

If you did not deploy during `oh init`:

```bash
# Deploy to a specific project
oh deploy MY-APP
```


| Target | Files generated in the project |
|--------|-------------------------------|
| `opencode` | `.opencode/agents/*.md` |

---

## 4. Launch the tool

```bash
oh start MY-APP
```

Launches the default tool (defined in `config/hub.json`) in the project directory.

With a startup prompt:

```bash
oh start MY-APP "explain the project architecture"
```

In development mode (loads open `ai-delegated` tickets):

```bash
oh start MY-APP --dev
```

With the terminal kanban board open in a second pane:

```bash
oh beads board MY-APP            # display the board once
oh beads board MY-APP --watch    # live refresh every 5s
```

---

## 5. Verify the deployment

```bash
oh deploy --check opencode MY-APP
```

Shows for each agent: `✓ UP TO DATE`, `⚠ OUTDATED` or `✗ MISSING`.

After a `git pull` on the hub (or `oh update`):

```bash
oh sync            # redeploys on all projects
oh sync --dry-run  # checks without deploying
```

---

## Expected result

At the end of these steps, in your project directory:

```
my-app/
└── .opencode/
    └── agents/
        ├── orchestrator.md
        ├── planner.md
        ├── reviewer.md
        ├── qa-engineer.md
        ├── debugger.md
        ├── auditor.md
        ├── developer-frontend.md
        └── ...
```

You can now invoke any agent in OpenCode:
- `"Implement the user login feature"` → `orchestrator` agent
- `"Audit the project security"` → `auditor` agent (security domain)
- `"Plan the payment module"` → `planner` agent

---

## Update the hub

### Update installed tools

```bash
oh update
```

Updates opencode, Beads, Beads UI, and external skills. If skills are modified, offers to re-run `oh sync`.

### Upgrade hub sources

```bash
oh upgrade
```

Pulls the latest hub scripts and agents (`git pull`). Offers to re-run `oh sync` after a successful update.

To switch to a specific version:

```bash
oh upgrade v1.1.0
```

Equivalent to the one-liner:

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | VERSION=v1.1.0 bash
```

---

## Troubleshooting

| Symptom | Solution |
|---------|----------|
| `oc: command not found` | Re-run `source ~/.zshrc` (or `~/.bashrc`) after installation |
| `curl: command not found` | Install curl, then re-run the one-liner |
| `Node.js not found` | Re-run `oh install` — offers available installers |
| Agent missing in the tool | Re-run `oh deploy MY-APP` |
| Outdated agent (`⚠ OUTDATED`) | `oh deploy MY-APP` to resynchronise |
| `bd: command not found` | Install Beads: `brew install beads` |
| Install directory already exists | `OPENCODE_HUB_DIR=~/other-path bash install.sh` |

---

## Uninstall the hub

```bash
oh uninstall
# or from anywhere:
bash ~/.openhub/uninstall.sh
```

The script guides the uninstallation through 4 optional steps (all with confirmation):

| Step | Action | Default |
|------|--------|---------|
| 1 | Clean up deployed agents in projects | `[y/N]` |
| 2 | Remove `~/.openhub` | `[y/N]` |
| 3 | Remove the alias and bun exports from the rc file | `[Y/n]` |
| 4 | Uninstall opencode, Beads, bun (separately) | `[y/N]` |
