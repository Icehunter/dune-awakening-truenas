# market-bot

An automated market bot for the Dune Awakening private server exchange. It runs as a k3s Deployment on the TrueNAS VM and continuously maintains sell listings for every tradeable item in the game, repriced dynamically based on sales velocity.

## How it works

Every 5 minutes the bot:

1. Loads the item catalog from `item-data.json` + `dune-item-names.json`
2. Queries the exchange for its own existing listings (bot actor: **Revy**, actor ID looked up dynamically)
3. Removes listings whose price has drifted from the current target
4. Tops up any partially-depleted stacks to full `stack_max`
5. Creates **5 listings per quality grade** for each applicable item
6. Refreshes all order expiry times to game-time + 24 h

The bot only touches its own NPC orders — player listings are never modified.

### Grade listings

Items that can drop from overland testing stations (ecolabs) are listed at **each of grades 0–5**, for 30 listings per item. Grade eligibility is determined by the `is_gradeable` flag in `item-data.json`, which is computed from CDT_BaseItems item tags during the `build-item-data.sh` pipeline — any non-schematic, non-resource item with a `LootTier.*` tag is considered gradeable.

Items without a `LootTier` tag (crafted-only, story-progression) are listed at **grade 0 only**, 5 listings. Schematics and stackable materials are always grade 0 only.

### Pricing

Base price = `vendor_price × rarity_multiplier`:

| Rarity | Multiplier |
|--------|-----------|
| Common | 2× |
| Unique | 3× |
| Memento | 5× |

Prices adjust ±5–10 % per tick based on how much of each item sold. Floor = base price; ceiling = 5× base price.

### Exchange details

| Setting | Value |
|---------|-------|
| Exchange | HarkoVillage\_EX (ID 2) |
| Access point | HarkoVillage\_AP (ID 1) |
| Bot character | Revy (class `Revy`, actor ID **varies per server** — looked up dynamically via `SELECT id FROM dune.actors WHERE class = 'Revy'`) |
| Listings per gradeable item | 5 per grade × 6 grades (0–5) = 30 total |
| Listings per non-gradeable item | 5 (grade 0 only) |
| Order expiry | 24 h (game time) |

---

## Architecture

```
your machine
  └─ deploy.sh
       ├─ cross-compiles market-bot-linux (GOOS=linux GOARCH=amd64)
       ├─ scp → VM /opt/market-bot/bin/market-bot
       ├─ scp → VM /opt/market-bot/data/{item-data,dune-item-names}.json
       └─ kubectl apply + rollout restart

TrueNAS VM (k3s)
  └─ Deployment: market-bot  (namespace: dune-market-bot)
       ├─ binary: hostPath /opt/market-bot/bin/market-bot
       ├─ data:   hostPath /opt/market-bot/data/
       └─ cache:  hostPath /opt/market-bot/cache/  (SQLite)
            └─ connects to PostgreSQL in funcom-seabass-* namespace
```

The bot runs entirely inside the cluster. `deploy.sh` only needs SSH access to the VM to upload files — it does not need direct database access.

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.21+ | Required for building locally |
| SSH key | — | Pre-installed on VM; auto-detected by `deploy.sh` |
| VM access | port 22 | To run `deploy.sh` |
| k3s cluster | running | Must have the Dune server Deployment active |
| VM OS | Alpine Linux | The target VM runs Alpine — `openssh-sftp-server` must be installed (see below) |

### SSH key

`deploy.sh` reads the key from `<repo-root>/sshKey` (one directory above `market-bot/`). This file is gitignored and managed separately per deployment.

---

## Item data

`dune-admin/item-data.json` and `dune-admin/dune-item-names.json` are committed to the repository. `deploy.sh` uploads them to the VM automatically — no extra steps needed.

---

## Building locally

All platforms require Go 1.21+. The binary is always cross-compiled for Linux/amd64 since it runs on the TrueNAS VM.

### macOS

```bash
brew install go   # if not already installed

cd market-bot
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o market-bot-linux .
```

### Linux (Ubuntu/Debian)

```bash
sudo apt-get update && sudo apt-get install -y golang-go

cd market-bot
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o market-bot-linux .
```

### Windows

```powershell
# Install Go from https://go.dev/dl/
cd market-bot
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -trimpath -ldflags="-s -w" -o market-bot-linux .
```

---

## Deploying

### macOS / Linux

```bash
cd market-bot
bash deploy.sh
```

Override the target IP:

```bash
DUNE_VM_IP=10.0.0.5 bash deploy.sh
```

### Windows (Hyper-V)

On Windows the server runs in a Hyper-V VM. Find its IP first:

```powershell
Get-VM | Select-Object Name, @{n='IP';e={($_ | Get-VMNetworkAdapter).IPAddresses[0]}}
```

Then deploy:

```powershell
cd market-bot
.\deploy.ps1 -VmIp 172.28.144.1
# or set it once:
$env:DUNE_VM_IP = "172.28.144.1"
.\deploy.ps1
```

`deploy.ps1` requires PowerShell 5.1+ and OpenSSH (built into Windows 10/11). No additional tools needed.

### What the deploy scripts do

Both scripts perform the same steps:
1. Cross-compiles the bot for Linux/amd64
2. Uploads the binary to `/opt/market-bot/bin/market-bot` on the VM (via `/tmp` to avoid permission errors)
3. Uploads `item-data.json` and `dune-item-names.json` to `/opt/market-bot/data/`
4. Applies the k8s manifest (`k8s/market-bot.yaml`)
5. Rolls out the Deployment and waits for it to become ready
6. Tails the last 40 log lines

### First-time deploy

#### VM prerequisites (Alpine Linux)

The target VM runs Alpine Linux. Before the first deploy, SSH in and ensure the SFTP subsystem is available — SCP will fail with `sftp-server: No such file or directory` without it:

```bash
sudo apk add openssh-sftp-server
sudo sed -i 's|^#\?Subsystem sftp .*|Subsystem sftp /usr/lib/ssh/sftp-server|' /etc/ssh/sshd_config
sudo rc-service sshd restart
```

#### Cluster setup

On a fresh VM the k3s Deployment and namespace do not yet exist. `deploy.sh` creates them via `kubectl apply`. The required directories (`/opt/market-bot/{bin,data,cache}`) are created automatically.

---

## Configuration

Static defaults live in `k8s/market-bot.yaml` (ConfigMap + Secret). The deploy scripts detect the database service and PostgreSQL pod in the target cluster during deployment, read the database connection values from the pod environment, render a temporary manifest, and apply that manifest. The detected password is not written back to `k8s/market-bot.yaml`.

You can override detection with environment variables:

```bash
DUNE_DB_HOST=... DUNE_DB_USER=... DUNE_DB_PASS=... bash deploy.sh
```

`DUNE_DB_PORT` and `DUNE_DB_NAME` are optional overrides.

### All settings

| Setting | Default | Where |
|---------|---------|-------|
| **DB host** | detected by deploy script | ConfigMap `DB_HOST` |
| DB port | detected by deploy script, fallback `15432` | ConfigMap `DB_PORT` |
| DB user | detected by deploy script | ConfigMap `DB_USER` |
| DB password | detected by deploy script | Secret `DB_PASS` |
| DB name | `dune` | ConfigMap `DB_NAME` |
| Tick interval | `5m` | ConfigMap `INTERVAL` |
| Item data path | `/data/item-data.json` | ConfigMap `ITEM_DATA_PATH` |
| Names path | `/data/dune-item-names.json` | ConfigMap `ITEM_NAMES_PATH` |
| Cache DB path | `/cache/market-bot-cache.db` | ConfigMap `CACHE_DB_PATH` |

---

## Logs

```bash
# Via deploy.sh (shown automatically after deploy)
# Or directly:
ssh -i ../sshKey dune@192.168.0.72 \
  "sudo kubectl logs -n dune-market-bot -l app=market-bot --tail=50 -f"
```

Key log lines:

```
market-bot catalog: 975 listable items          ← items loaded from JSON
market-bot exchange inventory id: 1613          ← bot's exchange inventory
market-bot bot actor id: 158 (Revy)             ← NPC actor confirmed/created
market-bot game epoch learned: unix 1776...     ← game clock calibrated from player orders
market-bot tick: 4875 created, 0 topped up, 0 pruned, 0 errors
```

---

## Wiping bot listings

To remove all bot listings from the database (e.g. before a redeploy with new prices):

```sql
WITH bot AS (
  SELECT id FROM dune.actors WHERE class = 'Revy' LIMIT 1
),
del_so AS (
  DELETE FROM dune.dune_exchange_sell_orders
  WHERE order_id IN (
    SELECT id FROM dune.dune_exchange_orders
    WHERE owner_id = (SELECT id FROM bot) AND is_npc_order = TRUE
  )
),
del_o AS (
  DELETE FROM dune.dune_exchange_orders
  WHERE owner_id = (SELECT id FROM bot) AND is_npc_order = TRUE
  RETURNING item_id
)
DELETE FROM dune.items WHERE id IN (SELECT item_id FROM del_o);
```

> **Note:** The bot's actor ID is assigned dynamically and **varies per server instance** — never hardcode it. The query above looks it up by the `Revy` class name, which is always correct.

Run this from the **dune-admin** Database tab or any PostgreSQL client with access to the cluster.

After wiping, also delete the SQLite cache on the VM so the bot does not reuse stale category data:

```bash
ssh -i ../sshKey dune@192.168.0.72 \
  "sudo rm -f /opt/market-bot/cache/market-bot-cache.db"
```

Then restart the bot:

```bash
ssh -i ../sshKey dune@192.168.0.72 \
  "sudo kubectl rollout restart deployment/market-bot -n dune-market-bot"
```

---

## Tests

```bash
cd market-bot
go test ./...
```

The test suite verifies the category mask encoding (category → 32-bit market filter code) for known items. Run this after any changes to `pricing.go`.
