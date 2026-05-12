#!/bin/bash
# Dune Awakening Battlegroup Management Script
# For TrueNAS Scale / Linux / macOS

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_KEY="$SCRIPT_DIR/sshKey"
VM_USER="dune"
VM_IP="${DUNE_VM_IP:-192.168.0.72}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

# Ensure SSH key has correct permissions
chmod 600 "$SSH_KEY" 2>/dev/null || true

ssh_cmd() {
    ssh -o StrictHostKeyChecking=no -o LogLevel=QUIET -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"
}

ssh_tty() {
    ssh -t -o StrictHostKeyChecking=no -o LogLevel=QUIET -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"
}

check_vm_ip() {
    if [[ -z "$VM_IP" ]]; then
        echo -e "${RED}Error:${NC} VM IP not set. Set DUNE_VM_IP environment variable or edit this script."
        echo "Example: export DUNE_VM_IP=192.168.1.100"
        exit 1
    fi
}

cmd_list() {
    echo -e "${CYAN}Listing battlegroups...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup list"
}

cmd_status() {
    echo -e "${CYAN}Checking battlegroup status...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup status"
}

cmd_start() {
    echo -e "${CYAN}Starting battlegroup...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup start"
}

cmd_stop() {
    echo -e "${CYAN}Stopping battlegroup...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup stop"
}

cmd_restart() {
    echo -e "${CYAN}Restarting battlegroup...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup restart"
}

cmd_update() {
    echo -e "${CYAN}Checking for updates...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup update"
}

cmd_shell_vm() {
    echo -e "${CYAN}Opening SSH shell to VM (type 'exit' to return)...${NC}"
    ssh_tty
}

cmd_shell_pod() {
    local bg_prefix="funcom-seabass-"
    local namespaces
    namespaces=$(ssh_cmd "sudo kubectl get ns --no-headers -o custom-columns=NAME:.metadata.name | grep '^$bg_prefix'" || true)

    if [[ -z "$namespaces" ]]; then
        echo -e "${YELLOW}No battlegroup found.${NC}"
        return
    fi

    local ns_array=()
    while IFS= read -r ns; do
        [[ -n "$ns" ]] && ns_array+=("$ns")
    done <<< "$namespaces"

    local ns
    if [[ ${#ns_array[@]} -eq 1 ]]; then
        ns="${ns_array[0]}"
    else
        echo "Select battlegroup:"
        for i in "${!ns_array[@]}"; do
            echo "  $((i+1)). ${ns_array[$i]#$bg_prefix}"
        done
        read -rp "Choice (1-${#ns_array[@]}): " choice
        ns="${ns_array[$((choice-1))]}"
    fi

    local pods
    pods=$(ssh_cmd "sudo kubectl get pods -n '$ns' --no-headers -o custom-columns=NAME:.metadata.name")

    local pod_array=()
    while IFS= read -r pod; do
        [[ -n "$pod" ]] && pod_array+=("$pod")
    done <<< "$pods"

    echo "Select pod:"
    for i in "${!pod_array[@]}"; do
        echo "  $((i+1)). ${pod_array[$i]}"
    done
    read -rp "Choice (1-${#pod_array[@]}): " choice
    local pod="${pod_array[$((choice-1))]}"

    echo -e "${CYAN}Opening shell in $pod...${NC}"
    ssh_tty "sudo kubectl exec -it '$pod' -n '$ns' -- /bin/bash || sudo kubectl exec -it '$pod' -n '$ns' -- /bin/sh"
}

cmd_logs_export() {
    echo -e "${CYAN}Exporting battlegroup logs...${NC}"
    ssh_tty "/home/dune/.dune/bin/battlegroup logs-export"

    local timestamp
    timestamp=$(date +%Y-%m-%d_%H-%M-%S)
    local local_dir="$HOME/BattlegroupLogs/Battlegroup_$timestamp"
    mkdir -p "$local_dir"

    echo -e "${CYAN}Downloading logs...${NC}"
    ssh_cmd "tar -czf - -C /tmp/dune-bg-logs ." | tar -xzf - -C "$local_dir"
    echo -e "${GREEN}Logs saved to: $local_dir${NC}"
}

cmd_open_director() {
    local port
    port=$(ssh_cmd "sudo kubectl get svc -A -o jsonpath='{.items[*].spec.ports[?(@.port==11717)].nodePort}' 2>/dev/null" | tr -d "'")
    if [[ -z "$port" || ! "$port" =~ ^[0-9]+$ ]]; then
        echo -e "${YELLOW}Could not find Director port. Is the battlegroup running?${NC}"
        return
    fi
    echo -e "${GREEN}Director URL: http://${VM_IP}:${port}/${NC}"

    # Try to open in browser
    if command -v xdg-open &>/dev/null; then
        xdg-open "http://${VM_IP}:${port}/" 2>/dev/null &
    elif command -v open &>/dev/null; then
        open "http://${VM_IP}:${port}/"
    fi
}

cmd_open_filebrowser() {
    echo -e "${GREEN}File Browser URL: http://${VM_IP}:18888/${NC}"
    if command -v xdg-open &>/dev/null; then
        xdg-open "http://${VM_IP}:18888/" 2>/dev/null &
    elif command -v open &>/dev/null; then
        open "http://${VM_IP}:18888/"
    fi
}

show_menu() {
    echo ""
    echo -e "${CYAN}Dune Awakening Battlegroup Manager${NC}"
    echo -e "VM: ${VM_USER}@${VM_IP}"
    echo ""
    echo "  1. list              List all battlegroups"
    echo "  2. status            Show battlegroup status"
    echo "  3. start             Start the battlegroup"
    echo "  4. stop              Stop the battlegroup"
    echo "  5. restart           Restart the battlegroup"
    echo "  6. update            Check for updates"
    echo "  7. shell-vm          SSH into the VM"
    echo "  8. shell-pod         Shell into a pod"
    echo "  9. logs-export       Export battlegroup logs"
    echo " 10. open-director     Open Director web UI"
    echo " 11. open-filebrowser  Open file browser"
    echo " 12. quit              Exit"
    echo ""
}

main() {
    check_vm_ip

    # Direct command mode
    if [[ $# -gt 0 ]]; then
        case "$1" in
            list) cmd_list ;;
            status) cmd_status ;;
            start) cmd_start ;;
            stop) cmd_stop ;;
            restart) cmd_restart ;;
            update) cmd_update ;;
            shell-vm) cmd_shell_vm ;;
            shell-pod) cmd_shell_pod ;;
            logs-export) cmd_logs_export ;;
            open-director) cmd_open_director ;;
            open-filebrowser) cmd_open_filebrowser ;;
            *) echo "Unknown command: $1"; exit 1 ;;
        esac
        exit 0
    fi

    # Interactive menu mode
    while true; do
        show_menu
        read -rp "Select option (1-12): " choice
        case "$choice" in
            1) cmd_list ;;
            2) cmd_status ;;
            3) cmd_start ;;
            4) cmd_stop ;;
            5) cmd_restart ;;
            6) cmd_update ;;
            7) cmd_shell_vm ;;
            8) cmd_shell_pod ;;
            9) cmd_logs_export ;;
            10) cmd_open_director ;;
            11) cmd_open_filebrowser ;;
            12) echo "Goodbye!"; exit 0 ;;
            *) echo -e "${YELLOW}Invalid choice${NC}" ;;
        esac
    done
}

main "$@"
