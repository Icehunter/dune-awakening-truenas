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
