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

echo "==> Dropping existing bot orders..."
DB_INFO=$(vm_ssh "sudo kubectl get pods -A --no-headers | grep 'db-dbdepl-sts-0'" 2>/dev/null || true)
if [ -n "$DB_INFO" ]; then
  DB_NS=$(echo "$DB_INFO" | awk '{print $1}')
  DB_POD=$(echo "$DB_INFO" | awk '{print $2}')
  vm_ssh "sudo kubectl exec -n $DB_NS $DB_POD -- psql -U dune -h localhost -p 15432 -d dune -c \"
    DELETE FROM dune.items WHERE id IN (
      SELECT item_id FROM dune.dune_exchange_orders
      WHERE owner_id = 158 AND is_npc_order = TRUE AND item_id IS NOT NULL);
    DELETE FROM dune.dune_exchange_orders
      WHERE owner_id = 158 AND is_npc_order = TRUE;\""
  echo "    done."
else
  echo "    warn: DB pod not found, skipping order cleanup."
fi

echo "==> Uploading binary..."
vm_scp "${SCRIPT_DIR}/market-bot-linux" "${VM_USER}@${VM_IP}:/tmp/market-bot-new"
vm_ssh "sudo mv /tmp/market-bot-new ${REMOTE_DIR}/bin/market-bot && sudo chmod +x ${REMOTE_DIR}/bin/market-bot"

echo "==> Uploading item data..."
vm_scp "${SCRIPT_DIR}/../dune-admin/item-data.json"       "${VM_USER}@${VM_IP}:${REMOTE_DIR}/data/item-data.json"
vm_scp "${SCRIPT_DIR}/../dune-admin/dune-item-names.json" "${VM_USER}@${VM_IP}:${REMOTE_DIR}/data/dune-item-names.json"

echo "==> Applying k8s manifests..."
vm_ssh "sudo kubectl apply -f -" < "${SCRIPT_DIR}/k8s/market-bot.yaml"

echo "==> Restarting deployment..."
vm_ssh "sudo kubectl rollout restart deployment/market-bot -n dune-market-bot"
vm_ssh "sudo kubectl rollout status deployment/market-bot -n dune-market-bot --timeout=90s"

echo ""
echo "==> Logs:"
vm_ssh "sudo kubectl logs -n dune-market-bot -l app=market-bot --tail=40"
