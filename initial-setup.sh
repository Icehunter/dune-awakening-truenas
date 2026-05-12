#!/bin/bash
# Dune Awakening Server - TrueNAS Initial Setup
# This script sets up the VM after it's been created in TrueNAS UI
#
# Prerequisites:
#   1. dune-server.qcow2 converted to zvol and VM created in TrueNAS UI
#   2. VM is running and has obtained an IP address
#   3. This script, sshKey, and bootstrap-setup are in the same directory
#
# Usage: ./initial-setup.sh <VM_IP>
# Example: ./initial-setup.sh 192.168.0.72

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_KEY="$SCRIPT_DIR/sshKey"
VM_USER="dune"
BOOTSTRAP_SCRIPT="$SCRIPT_DIR/bootstrap-setup"
VM_IP="${DUNE_VM_IP:-192.168.0.72}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

# Ensure SSH key has correct permissions (no Windows line endings)
if command -v sed &>/dev/null; then
    sed -i.bak 's/\r$//' "$SSH_KEY" 2>/dev/null || sed -i '' 's/\r$//' "$SSH_KEY" 2>/dev/null || true
    rm -f "${SSH_KEY}.bak" 2>/dev/null || true
fi
chmod 600 "$SSH_KEY"

ssh_cmd() {
    ssh -o StrictHostKeyChecking=no -o LogLevel=QUIET -o ConnectTimeout=10 -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"
}

ssh_tty() {
    ssh -t -o StrictHostKeyChecking=no -o LogLevel=QUIET -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"
}

echo -e "${CYAN}Dune Awakening Server - Initial Setup${NC}"
echo -e "VM: ${VM_USER}@${VM_IP}"
echo ""

# Step 1: Test connectivity
echo -e "${CYAN}Step 1: Testing SSH connectivity...${NC}"
if ! ssh_cmd "echo 'Connected to VM: \$(hostname)'"; then
    echo -e "${RED}Error:${NC} Cannot connect to VM at $VM_IP"
    echo "Make sure:"
    echo "  - The VM is running"
    echo "  - The IP address is correct"
    echo "  - The SSH key matches what's configured in the VM"
    exit 1
fi
echo -e "${GREEN}SSH connection successful${NC}"
echo ""

# Step 2: Write settings.conf
echo -e "${CYAN}Step 2: Writing settings.conf...${NC}"
ssh_cmd "mkdir -p /home/dune/.dune && printf '\n\n\n${VM_IP}\n' > /home/dune/.dune/settings.conf"
echo -e "${GREEN}Settings written${NC}"
echo ""

# Step 3: Upload bootstrap script
echo -e "${CYAN}Step 3: Uploading bootstrap script...${NC}"
if [[ ! -f "$BOOTSTRAP_SCRIPT" ]]; then
    echo -e "${RED}Error:${NC} Bootstrap script not found at $BOOTSTRAP_SCRIPT"
    exit 1
fi
cat "$BOOTSTRAP_SCRIPT" | ssh_cmd "cat > /tmp/setup && sudo mv /tmp/setup /home/dune/.dune/bin/setup && sudo chmod +x /home/dune/.dune/bin/setup"
echo -e "${GREEN}Bootstrap uploaded${NC}"
echo ""

# Step 4: Run bootstrap (Steam download + k3s setup)
echo -e "${CYAN}Step 4: Running bootstrap setup...${NC}"
echo -e "${YELLOW}This will download game files from Steam and set up k3s.${NC}"
echo -e "${YELLOW}This may take 10-30 minutes depending on your internet speed.${NC}"
echo ""
ssh_tty "/home/dune/.dune/bin/setup"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Initial setup complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Run battlegroup.sh to manage your server:"
echo "     DUNE_VM_IP=$VM_IP ./battlegroup.sh"
echo ""
echo "  2. Start your battlegroup:"
echo "     Select option 3 (start) from the menu"
echo ""
