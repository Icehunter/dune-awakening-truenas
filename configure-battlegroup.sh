#!/bin/bash
# Dune Awakening Battlegroup Configuration Script
# Configures sietches and map settings using kubectl patch (safe, preserves YAML anchors)
#
# Usage: ./configure-battlegroup.sh <VM_IP> [options]
#
# Options:
#   --sietches N        Number of Survival_1 sietches (default: 1, max: 25)
#   --deep-desert       Enable DeepDesert always-on (replicas=1)
#   --social-hubs       Enable Social Hubs always-on (replicas=1)
#   --dry-run           Show changes without applying
#   --save              Export live config to YAML file after patching

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_KEY="$SCRIPT_DIR/sshKey"
VM_USER="dune"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

# Defaults
SIETCHES=0  # 0 means don't change
DEEP_DESERT=false
SOCIAL_HUBS=false
DRY_RUN=false
SAVE=false
VM_IP=""

# Sietch names by dimension index
SIETCH_NAMES=(
    "Abbir" "al-Mut" "Alraab" "Barkan" "Coanua" "Eaqrab" "Fajr" "Gara Kulon"
    "Hajar" "Jacurutu" "Kathib" "Khafash" "Legg" "Makab" "Nadir" "Rajifiri"
    "Ramal" "Rifana" "Saajid" "Sandrat" "Ta'lab" "Tabr" "Tharwa" "Umbu" "Yaracuwan"
)

usage() {
    echo "Usage: $0 <VM_IP> [options]"
    echo ""
    echo "Options:"
    echo "  --sietches N        Number of Survival_1 sietches (1-25)"
    echo "  --deep-desert       Enable DeepDesert always-on (replicas=1)"
    echo "  --social-hubs       Enable Social Hubs always-on (replicas=1)"
    echo "  --dry-run           Show changes without applying"
    echo "  --save              Export live config to YAML file after patching"
    echo ""
    echo "Examples:"
    echo "  $0 192.168.0.72 --sietches 4 --deep-desert --social-hubs"
    echo "  $0 192.168.0.72 --sietches 2 --dry-run"
    echo ""
    echo "Sietch names by count:"
    echo "  1: Abbir"
    echo "  2: Abbir, al-Mut"
    echo "  3: Abbir, al-Mut, Alraab"
    echo "  4: Abbir, al-Mut, Alraab, Barkan"
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --sietches)
            SIETCHES="$2"
            if [[ $SIETCHES -lt 1 || $SIETCHES -gt 25 ]]; then
                echo -e "${RED}Error:${NC} Sietches must be 1-25"
                exit 1
            fi
            shift 2
            ;;
        --deep-desert)
            DEEP_DESERT=true
            shift
            ;;
        --social-hubs)
            SOCIAL_HUBS=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --save)
            SAVE=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            if [[ -z "$VM_IP" ]]; then
                VM_IP="$1"
            else
                echo "Unknown option: $1"
                usage
            fi
            shift
            ;;
    esac
done

if [[ -z "$VM_IP" ]]; then
    echo -e "${RED}Error:${NC} VM IP required"
    usage
fi

if [[ $SIETCHES -eq 0 && "$DEEP_DESERT" == "false" && "$SOCIAL_HUBS" == "false" ]]; then
    echo -e "${RED}Error:${NC} No changes specified. Use --sietches, --deep-desert, or --social-hubs"
    usage
fi

# Fix SSH key permissions
chmod 600 "$SSH_KEY" 2>/dev/null || true

ssh_cmd() {
    ssh -o StrictHostKeyChecking=no -o LogLevel=QUIET -o ConnectTimeout=10 -i "$SSH_KEY" "${VM_USER}@${VM_IP}" "$@"
}

echo -e "${CYAN}Dune Awakening Battlegroup Configuration${NC}"
echo -e "VM: ${VM_USER}@${VM_IP}"
echo ""

# Discover battlegroup
echo -e "${CYAN}Discovering battlegroup...${NC}"
BG_INFO=$(ssh_cmd "ls /home/dune/.dune/*.yaml 2>/dev/null | grep -v secret | head -1 | xargs basename | sed 's/.yaml//'")
if [[ -z "$BG_INFO" ]]; then
    echo -e "${RED}Error:${NC} No battlegroup found"
    exit 1
fi

BG_NAME="$BG_INFO"
BG_NS="funcom-seabass-$BG_NAME"

echo -e "Battlegroup: ${GREEN}$BG_NAME${NC}"
echo -e "Namespace: ${GREEN}$BG_NS${NC}"
echo ""

# Calculate RAM requirements
RAM_SIETCH=12
RAM_DEEPDESERT=15
RAM_OVERMAP=2
RAM_SOCIAL=2

RAM_NEEDED=$((SIETCHES * RAM_SIETCH + RAM_OVERMAP + 10))
if [[ "$DEEP_DESERT" == "true" ]]; then
    RAM_NEEDED=$((RAM_NEEDED + RAM_DEEPDESERT))
fi
if [[ "$SOCIAL_HUBS" == "true" ]]; then
    RAM_NEEDED=$((RAM_NEEDED + RAM_SOCIAL * 2))
fi

echo -e "${CYAN}Configuration:${NC}"
if [[ $SIETCHES -gt 0 ]]; then
    echo "  Sietches: $SIETCHES"
    echo -n "    Names: "
    for ((i=0; i<SIETCHES; i++)); do
        if [[ $i -gt 0 ]]; then echo -n ", "; fi
        echo -n "${SIETCH_NAMES[$i]}"
    done
    echo ""
fi
echo "  DeepDesert: $DEEP_DESERT"
echo "  Social Hubs: $SOCIAL_HUBS"
echo ""
echo -e "${YELLOW}Estimated RAM needed: ~${RAM_NEEDED}GB${NC}"
echo ""

# Check current VM RAM
CURRENT_RAM=$(ssh_cmd "free -g | awk '/Mem:/{print \$2}'" 2>/dev/null || echo "?")
echo -e "Current VM RAM: ${CURRENT_RAM}GB"

if [[ "$CURRENT_RAM" != "?" && $RAM_NEEDED -gt $CURRENT_RAM ]]; then
    echo -e "${YELLOW}Warning:${NC} Estimated RAM ($RAM_NEEDED GB) exceeds VM RAM ($CURRENT_RAM GB)"
fi
echo ""

# Build kubectl patch commands
PATCHES=()

if [[ $SIETCHES -gt 0 ]]; then
    # Get current partition count
    CURRENT_SIETCHES=$(ssh_cmd "sudo kubectl get battlegroup $BG_NAME -n $BG_NS -o jsonpath='{.spec.database.template.spec.deployment.spec.worldPartitions[0].partitions}' | jq length" 2>/dev/null || echo "1")
    
    echo -e "Current sietches: $CURRENT_SIETCHES"
    
    if [[ $SIETCHES -gt $CURRENT_SIETCHES ]]; then
        # Add new dimensions
        for ((dim=CURRENT_SIETCHES; dim<SIETCHES; dim++)); do
            PART_ID=$((28 + dim))  # Start from 29 for new partitions
            PATCHES+=("{\"op\":\"add\",\"path\":\"/spec/database/template/spec/deployment/spec/worldPartitions/0/partitions/-\",\"value\":{\"dimension\":$dim,\"disable\":false,\"id\":$PART_ID,\"maxX\":1,\"maxY\":1,\"minX\":0,\"minY\":0}}")
        done
    fi
    
    # Build partition list for server set
    PARTITION_IDS="[1"
    for ((dim=1; dim<SIETCHES; dim++)); do
        PARTITION_IDS+=",$(( 28 + dim ))"
    done
    PARTITION_IDS+="]"
    
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/0/replicas\",\"value\":$SIETCHES}")
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/0/partitions\",\"value\":$PARTITION_IDS}")
fi

# Server set indices and their partition IDs (from database worldPartitions):
# 0: Survival_1 (partition 1, plus 29+ for additional sietches)
# 1: Overmap (partition 2)
# 2: SH_Arrakeen (partition 3)
# 3: SH_HarkoVillage (partition 4)
# 7: DeepDesert_1 (partition 8)
#
# CRITICAL: All server sets with replicas > 0 MUST have:
# 1. partitions array set (enables -PartitionIndex for stable server indices)
# 2. dedicatedScaling: false (prevents auto-shutdown when empty)
# Without these, servers will crash with "stable indices" errors or auto-stop.

if [[ "$DEEP_DESERT" == "true" ]]; then
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/7/replicas\",\"value\":1}")
    PATCHES+=("{\"op\":\"add\",\"path\":\"/spec/serverGroup/template/spec/sets/7/partitions\",\"value\":[8]}")
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/7/dedicatedScaling\",\"value\":false}")
fi

if [[ "$SOCIAL_HUBS" == "true" ]]; then
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/2/replicas\",\"value\":1}")
    PATCHES+=("{\"op\":\"add\",\"path\":\"/spec/serverGroup/template/spec/sets/2/partitions\",\"value\":[3]}")
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/2/dedicatedScaling\",\"value\":false}")
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/3/replicas\",\"value\":1}")
    PATCHES+=("{\"op\":\"add\",\"path\":\"/spec/serverGroup/template/spec/sets/3/partitions\",\"value\":[4]}")
    PATCHES+=("{\"op\":\"replace\",\"path\":\"/spec/serverGroup/template/spec/sets/3/dedicatedScaling\",\"value\":false}")
fi

# Join patches
PATCH_JSON="["
for ((i=0; i<${#PATCHES[@]}; i++)); do
    if [[ $i -gt 0 ]]; then PATCH_JSON+=","; fi
    PATCH_JSON+="${PATCHES[$i]}"
done
PATCH_JSON+="]"

if [[ "$DRY_RUN" == "true" ]]; then
    echo -e "${YELLOW}DRY RUN - would apply this patch:${NC}"
    echo "$PATCH_JSON" | jq .
    exit 0
fi

# Apply patches
echo -e "${CYAN}Applying kubectl patches...${NC}"
ssh_cmd "sudo kubectl patch battlegroup $BG_NAME -n $BG_NS --type=json -p='$PATCH_JSON'"

# Recreate ServerSets for add-on servers to pick up new partition config
# The controller doesn't always reconcile properly, so we force it
if [[ "$DEEP_DESERT" == "true" || "$SOCIAL_HUBS" == "true" ]]; then
    echo -e "${CYAN}Recreating ServerSets to apply partition config...${NC}"
    sleep 5  # Wait for patch to propagate
    
    if [[ "$DEEP_DESERT" == "true" ]]; then
        ssh_cmd "sudo kubectl delete serverset -n $BG_NS ${BG_NAME}-sg-deepdesert-1 2>/dev/null" || true
    fi
    if [[ "$SOCIAL_HUBS" == "true" ]]; then
        ssh_cmd "sudo kubectl delete serverset -n $BG_NS ${BG_NAME}-sg-sh-arrakeen 2>/dev/null" || true
        ssh_cmd "sudo kubectl delete serverset -n $BG_NS ${BG_NAME}-sg-sh-harkovillage 2>/dev/null" || true
    fi
    
    echo "Waiting for ServerSets to recreate..."
    sleep 15
fi

# Save config if requested
if [[ "$SAVE" == "true" ]]; then
    echo -e "${CYAN}Saving config to YAML file...${NC}"
    ssh_cmd "sudo kubectl get battlegroup $BG_NAME -n $BG_NS -o yaml > /home/dune/.dune/$BG_NAME.yaml"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Configuration applied successfully!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Check status:"
echo "  ssh dune@$VM_IP 'sudo kubectl get battlegroup -n $BG_NS'"
echo ""
echo "View Director UI:"
echo "  http://$VM_IP:32067/"
