#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_KEY="${SCRIPT_DIR}/../sshKey"
VM_USER="dune"
REMOTE_DIR="/opt/market-bot"
DEPLOY_CONFIG="${SCRIPT_DIR}/.deploy-config"
MANIFEST_PATH="${SCRIPT_DIR}/k8s/market-bot.yaml"

# ── Load / prompt for VM IP ───────────────────────────────────────────────────
VM_IP="${DUNE_VM_IP:-}"

if [ -f "$DEPLOY_CONFIG" ]; then
  CACHED_IP=$(grep '^DUNE_VM_IP=' "$DEPLOY_CONFIG" 2>/dev/null | cut -d= -f2- | tr -d '[:space:]' || true)
  [ -n "$CACHED_IP" ] && VM_IP="$CACHED_IP"
fi

if [ -z "$VM_IP" ]; then
  printf "Enter VM IP address: "
  read -r VM_IP
  if [ -z "$VM_IP" ]; then
    echo "error: VM IP is required" >&2; exit 1
  fi
  echo "DUNE_VM_IP=${VM_IP}" >> "$DEPLOY_CONFIG"
  echo "    cached VM IP: $VM_IP"
fi

vm_ssh() { ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"; }
vm_scp() { scp -o StrictHostKeyChecking=no -i "$SSH_KEY" "$@"; }

manifest_value() {
  local key="$1"
  awk -v key="$key" '
    $1 == key ":" {
      sub(/^[^:]+:[[:space:]]*/, "")
      gsub(/^"|"$/, "")
      print
      exit
    }
  ' "$MANIFEST_PATH"
}

yaml_quote() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}

shell_quote() {
  printf "'"
  printf "%s" "$1" | sed "s/'/'\\\\''/g"
  printf "'"
}

set_manifest_value() {
  local file="$1"
  local key="$2"
  local value
  value="$(yaml_quote "$3")"
  YAML_KEY="$key" YAML_VALUE="$value" perl -0pi -e '
    my $key = $ENV{YAML_KEY};
    my $value = $ENV{YAML_VALUE};
    my $count = s/^(\s*\Q$key\E:\s*).*$/$1$value/m;
    die "missing manifest key: $key\n" unless $count;
  ' "$file"
}

kv_value() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key { sub(/^[^=]+=/, ""); print; exit }'
}

echo "==> Deploying to ${VM_IP}..."

# ── Cross-compile ─────────────────────────────────────────────────────────────
echo "==> Cross-compiling for Linux/amd64..."
cd "${SCRIPT_DIR}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o market-bot-linux .
echo "    built: $(du -sh market-bot-linux | cut -f1)"

# ── Remote directories ────────────────────────────────────────────────────────
echo "==> Preparing remote directories..."
vm_ssh "sudo mkdir -p ${REMOTE_DIR}/{data,cache,bin} && sudo chown -R ${VM_USER}:${VM_USER} ${REMOTE_DIR}"

# ── Detect DB connection details ──────────────────────────────────────────────
echo "==> Detecting DB connection details from cluster..."
DB_SVC_LINE=$(vm_ssh "sudo kubectl get svc -A --no-headers 2>/dev/null | awk '\$2 ~ /db-dbdepl-svc$/ { print; exit }'" 2>/dev/null || true)
DB_POD_LINE=$(vm_ssh "sudo kubectl get pods -A --no-headers 2>/dev/null | awk '\$2 ~ /db-dbdepl-sts-0$/ { print; exit }'" 2>/dev/null || true)

DETECTED_DB_HOST=""
DETECTED_DB_USER=""
DETECTED_DB_PASS=""
DETECTED_DB_NAME=""
DETECTED_DB_PORT=""
DB_NS=""
DB_POD=""

if [ -n "$DB_SVC_LINE" ]; then
  DB_SVC_NS=$(echo "$DB_SVC_LINE"  | awk '{print $1}')
  DB_SVC_NAME=$(echo "$DB_SVC_LINE" | awk '{print $2}')
  DETECTED_DB_HOST="${DB_SVC_NAME}.${DB_SVC_NS}.svc.cluster.local"
  echo "    host: $DETECTED_DB_HOST"
else
  echo "    warn: DB service not found, using DB_HOST from k8s/market-bot.yaml"
fi

if [ -n "$DB_POD_LINE" ]; then
  DB_NS=$(echo "$DB_POD_LINE" | awk '{print $1}')
  DB_POD=$(echo "$DB_POD_LINE" | awk '{print $2}')
  DB_ENV=$(vm_ssh "sudo kubectl exec -n ${DB_NS} ${DB_POD} -- printenv" 2>/dev/null || true)
  DETECTED_DB_USER=$(printf '%s\n' "$DB_ENV" | kv_value POSTGRES_USER)
  DETECTED_DB_PASS=$(printf '%s\n' "$DB_ENV" | kv_value POSTGRES_PASSWORD)
  DETECTED_DB_NAME=$(printf '%s\n' "$DB_ENV" | kv_value POSTGRES_DB)
  DETECTED_DB_PORT=$(printf '%s\n' "$DB_ENV" | kv_value PGPORT)
else
  echo "    warn: DB pod not found, using DB credentials from k8s/market-bot.yaml"
fi

DB_HOST="${DUNE_DB_HOST:-${DETECTED_DB_HOST:-$(manifest_value DB_HOST)}}"
DB_PORT="${DUNE_DB_PORT:-${DETECTED_DB_PORT:-$(manifest_value DB_PORT)}}"
DB_USER="${DUNE_DB_USER:-${DETECTED_DB_USER:-$(manifest_value DB_USER)}}"
DB_PASS="${DUNE_DB_PASS:-${DETECTED_DB_PASS:-$(manifest_value DB_PASS)}}"
DB_NAME="${DUNE_DB_NAME:-$(manifest_value DB_NAME)}"

[ -n "$DB_PORT" ] || DB_PORT="15432"
[ -n "$DB_NAME" ] || DB_NAME="${DETECTED_DB_NAME:-dune}"

if [ -z "$DB_HOST" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASS" ]; then
  echo "error: could not detect complete DB credentials. Set DUNE_DB_HOST, DUNE_DB_USER, DUNE_DB_PASS, and optionally DUNE_DB_PORT/DUNE_DB_NAME." >&2
  exit 1
fi

echo "    user: $DB_USER"
echo "    database: $DB_NAME"
echo "    port: $DB_PORT"
echo "    password: detected"

# ── Drop existing bot orders ──────────────────────────────────────────────────
echo "==> Dropping existing bot orders..."
if [ -n "$DB_NS" ] && [ -n "$DB_POD" ]; then
  DB_PASS_Q=$(shell_quote "$DB_PASS")
  DB_USER_Q=$(shell_quote "$DB_USER")
  DB_PORT_Q=$(shell_quote "$DB_PORT")
  DB_NAME_Q=$(shell_quote "$DB_NAME")
  printf '%s' "
WITH bot AS (SELECT id FROM dune.actors WHERE class = 'Revy' LIMIT 1),
del_orders AS (
  DELETE FROM dune.dune_exchange_orders
  WHERE owner_id = (SELECT id FROM bot) AND is_npc_order = TRUE
  RETURNING item_id
),
del_items AS (
  DELETE FROM dune.items
  WHERE id IN (SELECT item_id FROM del_orders WHERE item_id IS NOT NULL)
  RETURNING id
)
SELECT
  (SELECT COUNT(*) FROM del_orders) AS orders_deleted,
  (SELECT COUNT(*) FROM del_items)  AS items_deleted;
" | vm_ssh "sudo kubectl exec -n $DB_NS $DB_POD -i -- env PGPASSWORD=${DB_PASS_Q} psql -U ${DB_USER_Q} -h localhost -p ${DB_PORT_Q} -d ${DB_NAME_Q}"
else
  echo "    warn: DB pod not found, skipping order cleanup."
fi

# ── Upload ────────────────────────────────────────────────────────────────────
echo "==> Uploading binary..."
vm_scp "${SCRIPT_DIR}/market-bot-linux" "${VM_USER}@${VM_IP}:/tmp/market-bot-new"
vm_ssh "sudo mv /tmp/market-bot-new ${REMOTE_DIR}/bin/market-bot && sudo chmod +x ${REMOTE_DIR}/bin/market-bot"

echo "==> Uploading item data..."
vm_scp "${SCRIPT_DIR}/../dune-admin/item-data.json" "${VM_USER}@${VM_IP}:${REMOTE_DIR}/data/item-data.json"

# ── Apply k8s manifest ────────────────────────────────────────────────────────
echo "==> Applying k8s manifests..."
RENDERED_MANIFEST=$(mktemp)
trap 'rm -f "$RENDERED_MANIFEST"' EXIT
cp "$MANIFEST_PATH" "$RENDERED_MANIFEST"
set_manifest_value "$RENDERED_MANIFEST" DB_HOST "$DB_HOST"
set_manifest_value "$RENDERED_MANIFEST" DB_PORT "$DB_PORT"
set_manifest_value "$RENDERED_MANIFEST" DB_USER "$DB_USER"
set_manifest_value "$RENDERED_MANIFEST" DB_PASS "$DB_PASS"
set_manifest_value "$RENDERED_MANIFEST" DB_NAME "$DB_NAME"
vm_ssh "sudo kubectl apply -f -" < "$RENDERED_MANIFEST"

# ── Rollout ───────────────────────────────────────────────────────────────────
echo "==> Restarting deployment..."
vm_ssh "sudo kubectl rollout restart deployment/market-bot -n dune-market-bot"
vm_ssh "sudo kubectl rollout status deployment/market-bot -n dune-market-bot --timeout=90s" || true

echo ""
echo "==> Logs:"
vm_ssh "sudo kubectl logs -n dune-market-bot -l app=market-bot --tail=40"
