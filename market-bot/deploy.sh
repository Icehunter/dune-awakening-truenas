#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_KEY="${SCRIPT_DIR}/../sshKey"
VM_USER="dune"
VM_IP="${DUNE_VM_IP:-192.168.0.72}"
REMOTE_DIR="/opt/market-bot"

vm_ssh() { ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"; }
vm_scp() { scp -o StrictHostKeyChecking=no -i "$SSH_KEY" "$@"; }

echo "==> Cross-compiling for Linux/amd64..."
cd "${SCRIPT_DIR}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o market-bot-linux .
echo "    built: $(du -sh market-bot-linux | cut -f1)"

echo "==> Preparing remote directories..."
vm_ssh "sudo mkdir -p ${REMOTE_DIR}/{data,cache,bin} && sudo chown -R ${VM_USER}:${VM_USER} ${REMOTE_DIR}"

DEPLOY_CONFIG="${SCRIPT_DIR}/.deploy-config"

# Load cached config if present.
CACHED_DB_HOST=""
if [ -f "$DEPLOY_CONFIG" ]; then
  CACHED_DB_HOST=$(grep '^DUNE_DB_HOST=' "$DEPLOY_CONFIG" 2>/dev/null | cut -d= -f2- | tr -d '[:space:]' || true)
fi

echo "==> Detecting DB host from cluster..."
if [ -n "$CACHED_DB_HOST" ]; then
  DETECTED_DB_HOST="$CACHED_DB_HOST"
  echo "    using cached: $DETECTED_DB_HOST"
  echo "    (delete ${DEPLOY_CONFIG} to re-detect)"
else
  DB_SVC_LINE=$(vm_ssh "sudo kubectl get svc -A --no-headers 2>/dev/null | grep 'db-dbdepl-svc'" 2>/dev/null || true)
  if [ -n "$DB_SVC_LINE" ]; then
    DB_SVC_NS=$(echo "$DB_SVC_LINE"  | awk '{print $1}')
    DB_SVC_NAME=$(echo "$DB_SVC_LINE" | awk '{print $2}')
    DETECTED_DB_HOST="${DB_SVC_NAME}.${DB_SVC_NS}.svc.cluster.local"
    echo "DUNE_DB_HOST=${DETECTED_DB_HOST}" > "$DEPLOY_CONFIG"
    echo "    detected and cached: $DETECTED_DB_HOST"
  else
    DETECTED_DB_HOST=""
    echo "    warn: DB service not found, using value from k8s/market-bot.yaml"
  fi
fi

echo "==> Dropping existing bot orders..."
DB_POD_LINE=$(vm_ssh "sudo kubectl get pods -A --no-headers 2>/dev/null | grep 'db-dbdepl-sts-0'" 2>/dev/null || true)
if [ -n "$DB_POD_LINE" ]; then
  DB_NS=$(echo "$DB_POD_LINE" | awk '{print $1}')
  DB_POD=$(echo "$DB_POD_LINE" | awk '{print $2}')
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
" | vm_ssh "sudo kubectl exec -n $DB_NS $DB_POD -i -- psql -U dune -h localhost -p 15432 -d dune"
else
  echo "    warn: DB pod not found, skipping order cleanup."
fi

echo "==> Uploading binary..."
vm_scp "${SCRIPT_DIR}/market-bot-linux" "${VM_USER}@${VM_IP}:/tmp/market-bot-new"
vm_ssh "sudo mv /tmp/market-bot-new ${REMOTE_DIR}/bin/market-bot && sudo chmod +x ${REMOTE_DIR}/bin/market-bot"

echo "==> Uploading item data..."
vm_scp "${SCRIPT_DIR}/../dune-admin/item-data.json" "${VM_USER}@${VM_IP}:${REMOTE_DIR}/data/item-data.json"

echo "==> Applying k8s manifests..."
if [ -n "$DETECTED_DB_HOST" ]; then
  sed "s|DB_HOST:.*|DB_HOST: ${DETECTED_DB_HOST}|" "${SCRIPT_DIR}/k8s/market-bot.yaml" \
    | vm_ssh "sudo kubectl apply -f -"
else
  vm_ssh "sudo kubectl apply -f -" < "${SCRIPT_DIR}/k8s/market-bot.yaml"
fi

echo "==> Restarting deployment..."
vm_ssh "sudo kubectl rollout restart deployment/market-bot -n dune-market-bot"
vm_ssh "sudo kubectl rollout status deployment/market-bot -n dune-market-bot --timeout=90s" || true

echo ""
echo "==> Logs:"
vm_ssh "sudo kubectl logs -n dune-market-bot -l app=market-bot --tail=40"
