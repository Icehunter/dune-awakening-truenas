# dune-admin

A terminal UI (TUI) admin panel for managing a Dune Awakening private server. Connects to the server VM over SSH, then port-forwards into the k3s cluster to reach the PostgreSQL database and kubectl API directly from your local machine.

## Tabs

| Tab | What it does |
|-----|-------------|
| **Battlegroup** | Start/stop the battlegroup, view pod status, stream server logs |
| **Database** | Browse tables, describe schema, run raw SQL, search rows |
| **Logs** | Stream live logs from any pod in the cluster |
| **Players** | View connected players, character info |

## Architecture

```
your machine
  └─ SSH tunnel to VM (192.168.0.72:22)
       ├─ kubectl exec / port-forward → k3s cluster
       └─ PostgreSQL port-forward → dune DB (port 15432)
```

The app never exposes the database port publicly — all traffic flows through the SSH tunnel established at startup.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.21+ | `go.toolchain` in go.mod pins the exact version |
| SSH key | — | Pre-installed on the server VM; path auto-detected (see below) |
| Access to VM | port 22 | The VM must be reachable from your machine |

> **Item data (optional).** Pass `-itemdata ../dune-admin/item-data.json` to enable `stack_max` / `volume` display in the Database tab. Without it the tool still works — those columns just show raw DB values.

### SSH key lookup order

The app searches these paths in order and uses the first one found:

1. Value of `-key` flag (explicit override)
2. `../sshKey` (sibling to the repo's `dune-admin/` directory — the standard location)
3. `./sshKey` (current working directory)
4. `~/.ssh/dune`
5. `~/.ssh/id_ed25519`
6. `~/.ssh/id_rsa`

The SSH key is never committed to the repository (`.gitignore` blocks it). Whoever sets up the server places the key at `<repo-root>/sshKey`.

---

## Building

All platforms require Go 1.21+. Install from https://go.dev/dl/ or via a package manager.

### macOS

```bash
# Install Go via Homebrew (if not already installed)
brew install go

cd dune-admin
go build -o dune-admin .
./dune-admin
```

### Linux (Ubuntu/Debian)

```bash
# Install Go
sudo apt-get update && sudo apt-get install -y golang-go
# Or download the official tarball for a newer version:
# https://go.dev/dl/

cd dune-admin
go build -o dune-admin .
./dune-admin
```

### Windows

```powershell
# Install Go from https://go.dev/dl/ (installer or zip)
# Open a terminal in the dune-admin directory:

go build -o dune-admin.exe .
.\dune-admin.exe
```

> On Windows the TUI requires Windows Terminal or a modern terminal emulator for correct rendering. The classic `cmd.exe` prompt may not render box-drawing characters correctly.

---

## Running

```
./dune-admin [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | `192.168.0.72:22` | SSH host:port of the TrueNAS VM |
| `-user` | `dune` | SSH user on the VM |
| `-key` | *(auto-detected)* | Path to the SSH private key |
| `-itemdata` | *(empty)* | Path to `item-data.json` for stack/volume display |
| `-dbport` | `15432` | PostgreSQL port inside the k3s cluster |
| `-dbuser` | `dune` | PostgreSQL user |
| `-dbpass` | `dune` | PostgreSQL password |
| `-dbname` | `dune` | PostgreSQL database name |
| `-schema` | `dune` | PostgreSQL schema |
| `-scripcurrency` | `1` | Scrip currency ID (set `-1` for auto-detect) |

### Common invocations

```bash
# Standard — SSH key next to repo root, default server IP
./dune-admin

# Different server IP
./dune-admin -host 10.0.0.5:22

# Explicit key path
./dune-admin -key ~/.ssh/dune_server_key

# With item data for stack_max/volume display
./dune-admin -itemdata ../dune-admin/item-data.json
```

### Keyboard shortcuts (inside the TUI)

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch tabs |
| `↑` / `↓` | Navigate lists |
| `Enter` | Select / expand |
| `q` / `Ctrl+C` | Quit |
| `/` | Search (Database tab) |
| `Ctrl+S` | Export / save (Logs tab) |

---

## Item data (optional enrichment)

`item-data.json` is committed to the repository. It provides `stack_max`, `volume`, `tier`, `rarity`, `vendor_price`, and `category` for every tradeable item. Pass it via `-itemdata` to enable richer display in the Database tab — without it the tool still works fine, those columns just show raw DB values.
