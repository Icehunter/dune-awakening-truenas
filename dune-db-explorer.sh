#!/bin/bash
# Dune Awakening Database Explorer
# Run from VM: ~/dune-db-explorer.sh
# Or via: ./battlegroup.sh shell-vm then ~/dune-db-explorer.sh

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

# Database connection settings
DB_PORT=15432
DB_USER=dune
DB_NAME=dune
DB_SCHEMA=dune

# Find the database pod
find_db_pod() {
    local ns=$(sudo kubectl get ns --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null | grep '^funcom-seabass-' | head -1)
    if [[ -z "$ns" ]]; then
        echo ""
        return
    fi
    
    local pod=$(sudo kubectl get pods -n "$ns" --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null | grep -E 'db-dbdepl-sts' | head -1)
    echo "$ns:$pod"
}

# Run psql command in the database pod
run_psql() {
    local ns_pod=$(find_db_pod)
    if [[ -z "$ns_pod" || "$ns_pod" == ":" ]]; then
        echo -e "${RED}Error: Could not find database pod${NC}"
        return 1
    fi
    
    local ns="${ns_pod%%:*}"
    local pod="${ns_pod##*:}"
    
    sudo kubectl exec -n "$ns" "$pod" -- psql -h 127.0.0.1 -p $DB_PORT -U $DB_USER -d $DB_NAME -c "$1" 2>/dev/null
}

# Run psql interactively
run_psql_interactive() {
    local ns_pod=$(find_db_pod)
    if [[ -z "$ns_pod" || "$ns_pod" == ":" ]]; then
        echo -e "${RED}Error: Could not find database pod${NC}"
        return 1
    fi
    
    local ns="${ns_pod%%:*}"
    local pod="${ns_pod##*:}"
    
    echo -e "${CYAN}Connecting to database (type \\q to exit)...${NC}"
    sudo kubectl exec -it -n "$ns" "$pod" -- psql -h 127.0.0.1 -p $DB_PORT -U $DB_USER -d $DB_NAME
}

cmd_list_tables() {
    echo -e "${CYAN}=== All Tables (dune schema) ===${NC}"
    run_psql "SELECT tablename FROM pg_tables WHERE schemaname = '$DB_SCHEMA' ORDER BY tablename;"
}

cmd_table_sizes() {
    echo -e "${CYAN}=== Table Sizes (rows) ===${NC}"
    run_psql "
    SELECT 
        relname as table_name,
        n_live_tup as row_count
    FROM pg_stat_user_tables 
    ORDER BY n_live_tup DESC 
    LIMIT 30;
    "
}

cmd_describe_table() {
    local table="$1"
    if [[ -z "$table" ]]; then
        read -rp "Table name: " table
    fi
    echo -e "${CYAN}=== Structure of '$table' ===${NC}"
    run_psql "\\d+ $table"
}

cmd_sample_table() {
    local table="$1"
    local limit="${2:-10}"
    if [[ -z "$table" ]]; then
        read -rp "Table name: " table
    fi
    echo -e "${CYAN}=== Sample from '$table' (limit $limit) ===${NC}"
    run_psql "SELECT * FROM $table LIMIT $limit;"
}

cmd_find_player_tables() {
    echo -e "${CYAN}=== Player-related Tables ===${NC}"
    run_psql "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND (tablename ILIKE '%player%' OR tablename ILIKE '%character%' OR tablename ILIKE '%account%') ORDER BY tablename;"
}

cmd_find_item_tables() {
    echo -e "${CYAN}=== Item/Inventory Tables ===${NC}"
    run_psql "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND (tablename ILIKE '%item%' OR tablename ILIKE '%inventory%' OR tablename ILIKE '%loot%') ORDER BY tablename;"
}

cmd_find_building_tables() {
    echo -e "${CYAN}=== Building Tables ===${NC}"
    run_psql "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND (tablename ILIKE '%build%' OR tablename ILIKE '%place%' OR tablename ILIKE '%structure%' OR tablename ILIKE '%totem%' OR tablename ILIKE '%land%') ORDER BY tablename;"
}

cmd_find_progression_tables() {
    echo -e "${CYAN}=== Progression Tables ===${NC}"
    run_psql "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND (tablename ILIKE '%skill%' OR tablename ILIKE '%xp%' OR tablename ILIKE '%progress%' OR tablename ILIKE '%unlock%' OR tablename ILIKE '%recipe%' OR tablename ILIKE '%tech%') ORDER BY tablename;"
}

cmd_export_schema() {
    local outfile="${1:-/tmp/dune-schema.txt}"
    echo -e "${CYAN}Exporting full schema to $outfile...${NC}"
    
    local ns_pod=$(find_db_pod)
    local ns="${ns_pod%%:*}"
    local pod="${ns_pod##*:}"
    
    sudo kubectl exec -n "$ns" "$pod" -- pg_dump -U dune -d dune --schema-only 2>/dev/null > "$outfile"
    echo -e "${GREEN}Schema exported to $outfile${NC}"
    echo "Run: less $outfile"
}

cmd_search_columns() {
    local term="$1"
    if [[ -z "$term" ]]; then
        read -rp "Search term: " term
    fi
    echo -e "${CYAN}=== Columns matching '$term' ===${NC}"
    run_psql "
    SELECT 
        table_name, 
        column_name, 
        data_type 
    FROM information_schema.columns 
    WHERE table_schema = '$DB_SCHEMA' 
    AND (column_name ILIKE '%$term%' OR table_name ILIKE '%$term%')
    ORDER BY table_name, column_name;
    "
}

cmd_player_info() {
    echo -e "${CYAN}=== Player Characters ===${NC}"
    run_psql "SELECT a.id, a.class, a.map, a.gas_attributes FROM $DB_SCHEMA.actors a WHERE a.class LIKE '%PlayerCharacter%';"
}

cmd_player_currency() {
    echo -e "${CYAN}=== Player Currency (Solaris) ===${NC}"
    run_psql "SELECT * FROM $DB_SCHEMA.player_virtual_currency_balances;"
}

cmd_player_inventory() {
    local player_id="$1"
    if [[ -z "$player_id" ]]; then
        echo "Available players:"
        run_psql "SELECT id, class FROM $DB_SCHEMA.actors WHERE class LIKE '%PlayerCharacter%';"
        read -rp "Player actor ID: " player_id
    fi
    echo -e "${CYAN}=== Inventory for player $player_id ===${NC}"
    run_psql "
    SELECT i.template_id, i.stack_size, i.quality_level, i.stats 
    FROM $DB_SCHEMA.items i 
    JOIN $DB_SCHEMA.inventories inv ON i.inventory_id = inv.id 
    WHERE inv.actor_id = $player_id 
    ORDER BY i.template_id;
    "
}

cmd_give_currency() {
    local player_id="$1"
    local amount="$2"
    if [[ -z "$player_id" ]]; then
        echo "Available players:"
        run_psql "SELECT id, class FROM $DB_SCHEMA.actors WHERE class LIKE '%PlayerController%';"
        read -rp "Player controller ID: " player_id
    fi
    if [[ -z "$amount" ]]; then
        read -rp "Amount to add: " amount
    fi
    echo -e "${YELLOW}Adding $amount Solaris to player $player_id...${NC}"
    run_psql "UPDATE $DB_SCHEMA.player_virtual_currency_balances SET balance = balance + $amount WHERE player_controller_id = $player_id;"
    echo -e "${GREEN}Done! New balance:${NC}"
    run_psql "SELECT * FROM $DB_SCHEMA.player_virtual_currency_balances WHERE player_controller_id = $player_id;"
}

cmd_give_item() {
    local player_id="$1"
    local template="$2"
    local quantity="${3:-1}"
    
    if [[ -z "$player_id" ]]; then
        echo "Available players:"
        run_psql "SELECT id, class FROM $DB_SCHEMA.actors WHERE class LIKE '%PlayerCharacter%';"
        read -rp "Player character ID: " player_id
    fi
    if [[ -z "$template" ]]; then
        echo "Common items: Stone, MelangeSpice, SpiceResidue, SteelBar, Oil, ScrapMetal, Solaris"
        read -rp "Item template_id: " template
    fi
    
    echo -e "${YELLOW}Giving $quantity x $template to player $player_id...${NC}"
    # Get player's inventory ID
    local inv_id=$(run_psql "SELECT id FROM $DB_SCHEMA.inventories WHERE actor_id = $player_id LIMIT 1;" | grep -E '^\s*[0-9]+' | tr -d ' ')
    
    if [[ -z "$inv_id" ]]; then
        echo -e "${RED}Could not find inventory for player $player_id${NC}"
        return 1
    fi
    
    run_psql "INSERT INTO $DB_SCHEMA.items (inventory_id, stack_size, position_index, template_id, stats) VALUES ($inv_id, $quantity, 0, '$template', '{}');"
    echo -e "${GREEN}Done!${NC}"
}

cmd_interactive() {
    run_psql_interactive
}

show_menu() {
    echo ""
    echo -e "${CYAN}Dune Awakening Database Explorer${NC}"
    echo ""
    echo "  === EXPLORATION ==="
    echo "  1. list-tables        List all tables"
    echo "  2. table-sizes        Show table row counts"
    echo "  3. describe           Describe a table structure"
    echo "  4. sample             Sample rows from a table"
    echo "  5. search             Search for columns by name"
    echo ""
    echo "  === PLAYER INFO ==="
    echo "  6. player-info        Show player characters"
    echo "  7. player-currency    Show player Solaris balances"
    echo "  8. player-inventory   Show player inventory"
    echo ""
    echo "  === MODIFICATIONS ==="
    echo "  9. give-currency      Add Solaris to player"
    echo " 10. give-item          Add item to player inventory"
    echo ""
    echo " 11. export-schema      Export full schema to file"
    echo " 12. interactive        Open psql shell"
    echo " 13. quit               Exit"
    echo ""
}

main() {
    # Direct command mode
    if [[ $# -gt 0 ]]; then
        case "$1" in
            list-tables|list) cmd_list_tables ;;
            table-sizes|sizes) cmd_table_sizes ;;
            describe|desc) cmd_describe_table "$2" ;;
            sample) cmd_sample_table "$2" "$3" ;;
            search) cmd_search_columns "$2" ;;
            player-info|players) cmd_player_info ;;
            player-currency|currency) cmd_player_currency ;;
            player-inventory|inv) cmd_player_inventory "$2" ;;
            give-currency) cmd_give_currency "$2" "$3" ;;
            give-item) cmd_give_item "$2" "$3" "$4" ;;
            export-schema|export) cmd_export_schema "$2" ;;
            interactive|shell|psql) cmd_interactive ;;
            *) echo "Unknown command: $1"; exit 1 ;;
        esac
        exit 0
    fi

    # Interactive menu mode
    while true; do
        show_menu
        read -rp "Select option (1-13): " choice
        case "$choice" in
            1) cmd_list_tables ;;
            2) cmd_table_sizes ;;
            3) cmd_describe_table ;;
            4) cmd_sample_table ;;
            5) cmd_search_columns ;;
            6) cmd_player_info ;;
            7) cmd_player_currency ;;
            8) cmd_player_inventory ;;
            9) cmd_give_currency ;;
            10) cmd_give_item ;;
            11) cmd_export_schema ;;
            12) cmd_interactive ;;
            13) echo "Goodbye!"; exit 0 ;;
            *) echo -e "${YELLOW}Invalid choice${NC}" ;;
        esac
    done
}

main "$@"
