# Dune Awakening — Database Query Reference

All tables, stored procedures, functions, and query patterns from the 83 SQL files in
`images/battlegroup/server-db-utils-extracted/rootfs/root/DuneSandbox/Database/`.

---

## Table of Contents

1. [Core Infrastructure](#1-core-infrastructure)
2. [Accounts & Players](#2-accounts--players)
3. [Actors & World Partition](#3-actors--world-partition)
4. [Inventory & Items](#4-inventory--items)
5. [Dune Exchange (Market)](#5-dune-exchange-market)
6. [Buildings & Placeables](#6-buildings--placeables)
7. [Vehicles](#7-vehicles)
8. [Factions & Reputation](#8-factions--reputation)
9. [Specialization / XP](#9-specialization--xp)
10. [Journey / Quest System](#10-journey--quest-system)
11. [Map, Markers & Overmap](#11-map-markers--overmap)
12. [Spice Fields & Resource Fields](#12-spice-fields--resource-fields)
13. [Totems & Landclaim](#13-totems--landclaim)
14. [Vendors](#14-vendors)
15. [Communinet (In-game Chat)](#15-communinet-in-game-chat)
16. [Mnemonic Recall (Codex)](#16-mnemonic-recall-codex)
17. [Friends & Player Info Lookup](#17-friends--player-info-lookup)
18. [Dungeons](#18-dungeons)
19. [Vehicle Recovery](#19-vehicle-recovery)
20. [Tutorials](#20-tutorials)
21. [Lore Pickups](#21-lore-pickups)
22. [Event Log (Game Events)](#22-event-log-game-events)
23. [Event Logging (Solaris Audit)](#23-event-logging-solaris-audit)
24. [Sinkcharts](#24-sinkcharts)
25. [Building Blueprints & Favorites](#25-building-blueprints--favorites)
26. [Player Access Codes](#26-player-access-codes)
27. [Admin & Account Tools](#27-admin--account-tools)
28. [Cheat Detection](#28-cheat-detection)
29. [Character Transfers](#29-character-transfers)
30. [Demo / Trial Accounts](#30-demo--trial-accounts)
31. [Coriolis Storm](#31-coriolis-storm)
32. [Encounters & Spawners](#32-encounters--spawners)
33. [World Partition Presets](#33-world-partition-presets)
34. [Debug Utilities](#34-debug-utilities)
35. [Upgrade Migrations](#35-upgrade-migrations)

---

## 1. Core Infrastructure

**File:** `01_Dune.sql`

### Custom Types (selected key types)

| Type | Purpose |
|------|---------|
| `INVENTORYITEM` | Full item row (id, inventory_id, stack_size, template_id, stats, quality_level, …) |
| `BUILDINGINSTANCE` | Building piece with transform, health, shelter, stabilization state |
| `ActorGenericData` | Combined actor payload (FGL entities, properties, gas attrs, building/placeable data) |
| `ActorDescription` | Full actor save bundle (id, class, transform, generic_data, serial) |
| `PlayerConnectionStatus` | ENUM: `Offline`, `LoggingOut`, `Online` |
| `PlayerLifeState` | ENUM: `Alive`, `Dead`, `DeadByCoriolis`, `DeadBySandworm` |
| `SpecializationTrackType` | ENUM: `Invalid`, `Crafting`, `Gathering`, `Exploration`, `Combat`, `Sabotage`, `Count` |
| `DemoState` | ENUM: `Demo`, `DbMigratedToRetail`, `Retail` |
| `SpawnLocatorType` | ENUM: `Invalid`, `Transform`, `PersistentActor`, `StaticLocatorName` |

### Core Tables

| Table | Description |
|-------|-------------|
| `encrypted_accounts` | Account registry — user ID, encrypted Funcom ID, platform |
| `accounts` (view) | Decrypted view over `encrypted_accounts` |
| `farm_variables` | Singleton: universe time, farm ID, down-time accumulation, battlegroup close date |
| `farm_state` | Per-server state: connections, IPs, ports, map, revision, alive/ready flags |
| `world_partition` | Map partitions linked to servers; blocked flag; label for admin teleport |
| `actors` | All game actors: class, map, transform, partition_id, owner, serial, properties/gas_attributes |
| `fgl_entities` | Functional game-layer entity components (JSON payload) |
| `actor_fgl_entities` | Links actors to FGL entities by slot_name |

### Encryption Functions

```sql
encrypt_user_data(in_data text) RETURNS bytea      -- store plaintext as bytes
decrypt_user_data(in_encrypted_data bytea) RETURNS text  -- restore plaintext
get_stored_user_data_encryption_status() RETURNS UserDataEncryptionStatus
```

---

## 2. Accounts & Players

**Files:** `02_player.sql`, `20_proc_get_account_id_by_user.sql`, `82_account_takeover.sql`, `74_player_info.sql`

### Functions

```sql
-- Core account helpers
get_solaris_id() RETURNS SMALLINT                          -- always returns 0
get_account_actor_ids(in_account_id BIGINT) RETURNS PlayerActorIds  -- controller/state/pawn triple
delete_account(in_user_id text, in_reason text) RETURNS BOOLEAN
dune_get_account_id_by_user(in_user TEXT) RETURNS BIGINT

-- Player info lookups (multi-param overloads)
get_player_infos_for_actor_ids(in_actor_ids BIGINT[]) RETURNS TABLE(player_id, character_name, fls_id, funcom_id, platform_id, platform_name)
get_player_infos_for_fls_ids(in_fls_ids TEXT[])  -- same columns
get_player_infos_for_funcom_ids(in_funcom_ids TEXT[])
get_player_infos_for_character_names(in_character_names TEXT[])
get_controller_id_from_platform_id(in_platform_id TEXT) RETURNS BIGINT
perform_notify_on_character_delete(in_user_id TEXT)   -- fires pg_notify

-- Account takeover (NPC/shared account support)
load_takeoverable_user_ids() RETURNS SETOF TakeoverCharacterDataComposite
set_account_as_takeoverable(in_user_id TEXT, in_new_user_id TEXT)
can_takeover_account(in_user_id TEXT) RETURNS BOOLEAN
takeover_account(in_user_to_takeover TEXT, in_current_user TEXT)  -- swaps FLS IDs + funcom IDs
```

### Key Tables (player state)

| Table | Description |
|-------|-------------|
| `player_state` | character_name, player_controller_id, player_pawn_id, player_state_id, online_status, last_avatar_activity, server_id |
| `player_tags` | account_id + tag (TEXT) — used for faction tier, content unlocks, etc. |
| `player_virtual_currency_balances` | player_controller_id + currency_id + balance |
| `player_respawn_locations` | locator type + actor ID for respawn beacons |
| `account_removal_log` | Audit trail of deleted accounts |

### Key Queries (dune-admin uses these)

```sql
-- All player characters with faction
SELECT a.id, a.owner_account_id,
       COALESCE(ps.character_name, convert_from(e.encrypted_funcom_id,'UTF8'), ''),
       COALESCE(ps.player_controller_id, 0),
       a.class, a.map, COALESCE(pf.faction_id, 0)
FROM dune.actors a
LEFT JOIN dune.player_state ps ON ps.account_id = a.owner_account_id
LEFT JOIN dune.encrypted_accounts e ON e.id = a.owner_account_id
LEFT JOIN dune.player_faction pf ON pf.actor_id = a.id
WHERE a.class ILIKE '%PlayerCharacter%'
ORDER BY a.id;

-- Online / recently disconnected players
SELECT actors.id, player_state.character_name,
       (actors.map, actors.partition_id, actors.dimension_index)::ServerInfo,
       (player_state.last_avatar_activity AT TIME ZONE 'UTC')::TIMESTAMP,
       player_state.online_status
FROM actors
JOIN player_state ON player_state.player_controller_id = actors.id
WHERE player_state.online_status = 'Online'
   OR player_state.last_avatar_activity > NOW() - INTERVAL '1 minutes';

-- Give Solaris (currency_id = 0)
UPDATE dune.player_virtual_currency_balances
SET balance = balance + $1
WHERE player_controller_id = $2 AND currency_id = 0;

-- Give scrips (non-Solaris currency)
SELECT dune.adjust_player_virtual_currency_balance($actor_id, $currency_id, $delta);
```

---

## 3. Actors & World Partition

**Files:** `09_actors.sql`, `17_proc_world_partition.sql`

### Functions

```sql
-- Actor management
save_actor_dislocation(in_actor_id, in_current_server_info, in_target_location, in_target_dimension_index)
delete_actors(in_ids BIGINT[])
add_actor_audit(in_id BIGINT, in_class TEXT)

-- World partition
determine_partition_label(map, dimension_index, label, allow_collision) -- label generation
```

### Tables

| Table | Description |
|-------|-------------|
| `actor_audit` | id + class — tombstone records for deleted actors |
| `world_partition` | partition_id, server_id, map, partition_definition (JSONB), dimension_index, blocked, label |

---

## 4. Inventory & Items

**Files:** `05_inventory.sql`, `18_proc_advance_items_id_sequencer.sql`, `54_proc_remove_items_and_recipes.sql`

### Tables

| Table | Description |
|-------|-------------|
| `inventories` | id, actor_id, exchange_id, item_id, vehicle_module_id, inventory_type, max_item_count, max_item_volume |
| `actor_inventories` | inventory_id + component_name_hash |
| `items` | id, inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats (JSONB), quality_level, volume_override |
| `removed_items` | Template IDs of items removed by game patches |
| `removed_recipes` | Template IDs of recipes removed by patches |

### Functions

```sql
advance_items_id_sequencer(count BIGINT) RETURNS BIGINT   -- advisory-locked sequence reservation

remove_items_and_recipes(items_to_remove text[], recipes_to_remove text[])
get_items_to_remove(items_to_remove text[]) RETURNS text[]
get_recipes_to_remove(recipes_to_remove text[]) RETURNS text[]
remove_items(items text[])
remove_recipes_from_actor_properties(recipes text[])
remove_items_or_recipes_from_fgl_entities(ids text[])
update_removed_items_and_recipes(items text[], recipes text[])
```

### Key Queries (dune-admin uses these)

```sql
-- Player inventory
SELECT i.id, i.template_id, i.stack_size, i.quality_level,
       COALESCE((i.stats->'FItemStackAndDurabilityStats'->1->>'CurrentDurability'), 'N/A')
FROM dune.items i
JOIN dune.inventories inv ON i.inventory_id = inv.id
WHERE inv.actor_id = $player_id
ORDER BY i.template_id;

-- Find backpack inventory
SELECT id, COALESCE(max_item_count,-1), COALESCE(max_item_volume,-1)
FROM dune.inventories
WHERE actor_id = $1 AND inventory_type = 0 LIMIT 1;

-- Give item (new stack)
INSERT INTO dune.items (inventory_id, stack_size, position_index, template_id, quality_level, stats)
VALUES ($inv_id, $qty, $pos, $template, $quality, '{}');

-- Top up existing stack
UPDATE dune.items SET stack_size = stack_size + $1 WHERE id = $2;

-- Delete specific item
DELETE FROM dune.items WHERE id = $item_id;

-- Distinct templates currently in DB (for autocomplete)
SELECT DISTINCT template_id FROM dune.items ORDER BY template_id;
```

---

## 5. Dune Exchange (Market)

**Files:** `16_proc_dune_exchange.sql`, `01_Dune.sql` (tables)

### Tables

| Table | Description |
|-------|-------------|
| `dune_exchanges` | id, exchange_name, inventory_id — one per zone (e.g. "HarkoVillage_EX") |
| `dune_exchange_accesspoints` | id, exchange_id, name (e.g. "HarkoVillage_AP") |
| `dune_exchange_users` | id, owner_id (actor), solari_balance |
| `dune_exchange_orders` | Full order: exchange_id, owner_id, item_id, template_id, category_mask, category_depth, is_npc_order, item_price, expiration_time, durability_cur/max, quality_level |
| `dune_exchange_sell_orders` | order_id, initial_stack_size, wear_normalized_price |
| `dune_exchange_fulfilled_orders` | order_id, completion_type, stack_size, source_order_id, original_order_id |
| `dune_exchange_categories_hash` | id=1, hash — poisoned to 0 to force client to refresh |

### Functions

```sql
get_dune_exchange_used_order_slots(in_controller_id BIGINT) RETURNS INT
get_dune_exchange_data(in_exchange_id, in_controller_id) RETURNS LoadExchangeDataResult
get_dune_exchange_id(in_name TEXT) RETURNS BIGINT
get_exchange_inventory_id(in_exchange_id BIGINT) RETURNS BIGINT
dune_exchange_get_user_id(in_actor_id BIGINT) RETURNS BIGINT
dune_exchange_modify_user_solari_balance(in_actor_id BIGINT, in_delta BIGINT)
adjust_player_virtual_currency_balance(in_actor_id, in_currency_id, in_delta)
adjust_player_solaris_balance(in_actor_id, in_delta)
```

### Key Queries (market-bot uses these)

```sql
-- Detect exchange & access point
SELECT exchange_id FROM dune.dune_exchange_orders WHERE is_npc_order=FALSE LIMIT 1;
SELECT DISTINCT access_point_id FROM dune.dune_exchange_orders WHERE exchange_id=$1 LIMIT 1;

-- All bot's active listings
SELECT o.id, o.template_id, o.item_id, o.item_price, i.stack_size, o.quality_level
FROM dune.dune_exchange_orders o
JOIN dune.items i ON i.id = o.item_id
WHERE o.owner_id=$bot_id AND o.is_npc_order=TRUE;

-- Player listings to potentially buy
SELECT o.id, o.template_id, o.item_price, o.item_id, o.owner_id,
       COALESCE(i.stack_size, s.initial_stack_size), COALESCE(o.quality_level,0)
FROM dune.dune_exchange_orders o
JOIN dune.dune_exchange_sell_orders s ON s.order_id=o.id
LEFT JOIN dune.items i ON i.id=o.item_id
WHERE o.is_npc_order=FALSE AND o.exchange_id=$1 LIMIT $n;

-- Sales stats per template
SELECT o.template_id,
       COALESCE(SUM(f.stack_size),0) AS sold,
       COALESCE(MAX(s.initial_stack_size),0) AS listed
FROM dune.dune_exchange_orders o
JOIN dune.dune_exchange_sell_orders s ON s.order_id=o.id
LEFT JOIN dune.dune_exchange_fulfilled_orders f ON f.order_id=o.id
WHERE o.owner_id=$bot_id AND o.is_npc_order=TRUE
GROUP BY o.template_id;

-- Create NPC sell listing (insert item + order + sell_order)
INSERT INTO dune.items (...) VALUES (...) RETURNING id;
INSERT INTO dune.dune_exchange_orders (..., is_npc_order=TRUE, ...) VALUES (...) RETURNING id;
INSERT INTO dune.dune_exchange_sell_orders (order_id, initial_stack_size, wear_normalized_price)
VALUES ($1,$2,$3);

-- Wipe all bot listings
WITH bot AS (SELECT id FROM dune.actors WHERE class='Revy' LIMIT 1),
del_so AS (DELETE FROM dune.dune_exchange_sell_orders WHERE order_id IN (
  SELECT id FROM dune.dune_exchange_orders WHERE owner_id=(SELECT id FROM bot) AND is_npc_order=TRUE)),
del_o AS (DELETE FROM dune.dune_exchange_orders WHERE owner_id=(SELECT id FROM bot) AND is_npc_order=TRUE RETURNING item_id)
DELETE FROM dune.items WHERE id IN (SELECT item_id FROM del_o);

-- Poison category hash (forces client refresh)
INSERT INTO dune.dune_exchange_categories_hash (id, hash) VALUES (1,0)
ON CONFLICT (id) DO UPDATE SET hash=0;
```

---

## 6. Buildings & Placeables

**Files:** `04_building.sql`, `10_placeable.sql`, `32_proc_building_blueprint.sql`, `90_building_favorites.sql`, `55_proc_backup_vehicle.sql`

### Tables

| Table | Description |
|-------|-------------|
| `buildings` | id (FK actors), owner_id |
| `building_instances` | Per-piece data: building_id, instance_id, building_type, transform, owner_entity_id, flags, health, shelter, stabilization, sand_buildup |
| `placeables` | id (FK actors), owner_entity_id, health, building_type, has_hit_ground, has_buildable_support, is_hologram |
| `building_blueprints` | id, item_id, player_id, map |
| `building_blueprint_instances` | Blueprint piece data |
| `building_blueprint_placeables` | Blueprint placeable data |
| `building_blueprint_pentashields` | Pentashield scale data |
| `base_backups` | Named base backup sets |
| `base_backup_linked_actors` | backup_id → actor_id mapping |
| `building_favorites` | account_id → building_types TEXT[] |
| `building_progression` | account_id → learned_building_sets TEXT[], new_buildable_pieces TEXT[] |
| `tax_invoice` | Totem tax ledger: totem_id, reference_timespan, invoice_status, amount |

### Functions

```sql
-- Building blueprints
delete_building_blueprint(in_building_item_id BIGINT)
save_building_blueprint_copy(...)
get_building_blueprint_copy_data(in_building_blueprint_id BIGINT) RETURNS BuildingBlueprintGetCopyData

-- Building favorites / progression
get_building_favorites(in_account_id BIGINT) RETURNS TABLE(building_types TEXT[])
update_server_building_favorites(in_account_id, in_building_types TEXT[])
get_learned_building_sets(in_account_id BIGINT) RETURNS SETOF TEXT
update_server_learned_building_sets(in_account_id, in_learned_building_sets TEXT[])
get_learned_new_buildable_pieces(in_account_id BIGINT)
update_server_learned_new_buildable_pieces(in_account_id, in_new_buildable_pieces TEXT[])
```

### Key Queries

```sql
-- Count buildings + totems per player
SELECT COUNT(*) FROM dune.buildings b
JOIN dune.actors a ON b.id = a.id
WHERE a.owner_account_id = $account_id;

SELECT COUNT(*) FROM dune.totems t
JOIN dune.actors a ON t.id = a.id
WHERE a.owner_account_id = $account_id;
```

---

## 7. Vehicles

**Files:** `64_vehicles.sql`, `55_proc_backup_vehicle.sql`, `vehicle_recovery.sql`

### Tables

| Table | Description |
|-------|-------------|
| `vehicles` | id (FK actors) |
| `vehicle_modules` | id, vehicle_id, template_id, stats (JSONB) |
| `vehicle_module_inventories` | inventory_id + vehicle_module_inventory_type |
| `backup_vehicles` | account_id + vehicle_id + customization_id — one-slot emergency backup |
| `recovered_vehicles` | account_id, vehicle_id, time_stored, chassis_durability, vehicle_name, customization_id, migrated |

### Functions

```sql
-- Backup vehicle (1-slot emergency)
load_backup_vehicle(in_account_id BIGINT) RETURNS TABLE(id, class, customization_id)
store_backup_vehicle(in_vehicle_id, in_account_id, in_customization_id)
delete_backup_vehicle(in_account_id BIGINT)

-- Vehicle recovery (wrecks)
load_recovered_vehicles(in_account_id, in_restore_time_limit) RETURNS TABLE(vehicle_id, class, name, time_stored, chassis_durability, customization_id, migrated)
store_recovered_vehicle(in_vehicle_id, in_chassis_durability, in_customization_id, in_is_migration)
delete_recovered_vehicle(in_vehicle_id BIGINT)
get_all_recoverable_vehicles_for_account(in_account_id BIGINT)
```

---

## 8. Factions & Reputation

**Files:** `60_proc_factions.sql`

### Tables

| Table | Description |
|-------|-------------|
| `factions` | id (smallint), name TEXT |
| `player_faction` | actor_id, faction_id, utc_time_faction_change |
| `player_faction_reputation` | actor_id, faction_id, reputation_amount (INT) |
| `guild_members` | player_id, guild_id |

### Functions

```sql
register_new_factions(factions text[]) RETURNS TABLE(faction_id smallint, faction_name text)
change_player_faction(in_player_id, in_faction_id, neutral_faction_id, in_utc_time_faction_change)
get_player_faction(in_player_id, in_neutral_faction_id) RETURNS SMALLINT
get_player_faction_name(in_actor_id) OUT player_faction_name, utc_time_faction_change
get_all_faction_members() RETURNS TABLE(player_id, fls_id, faction_id)
get_player_current_faction_reputation(in_actor_id) OUT faction_id, reputation_amount
set_player_faction_reputation(in_actor_id, in_faction_id, in_reputation_amount)

-- Also: handle_player_faction_guild_effects() — called internally on faction change
```

### Key Queries (dune-admin uses these)

```sql
-- All faction reputations + scrip balance
SELECT pfr.actor_id, pfr.faction_id, f.name, pfr.reputation_amount,
       COALESCE(vcb.balance, 0)
FROM dune.player_faction_reputation pfr
JOIN dune.factions f ON f.id = pfr.faction_id
LEFT JOIN dune.player_virtual_currency_balances vcb
  ON vcb.player_controller_id = pfr.actor_id AND vcb.currency_id = $scrip_currency_id
ORDER BY pfr.actor_id, pfr.faction_id;

-- Upsert faction reputation
UPDATE dune.player_faction_reputation
SET reputation_amount = reputation_amount + $delta
WHERE actor_id = $actor_id AND faction_id = $faction_id;
-- (INSERT if no rows affected)

-- Faction tier tags (synced by dune-admin after rep change)
INSERT INTO dune.player_tags (account_id, tag) VALUES ($1, $2) ON CONFLICT DO NOTHING;
DELETE FROM dune.player_tags WHERE account_id=$1 AND tag=$2;
```

---

## 9. Specialization / XP

**Files:** `specialization_progression.sql`

### Tables

| Table | Description |
|-------|-------------|
| `specialization_tracks` | player_id, track_type (SpecializationTrackType enum), xp_amount, level (REAL) |
| `specialization_keystones_map` | id (smallint), name TEXT |
| `purchased_specialization_keystones` | player_id, keystone_id |

### Functions

```sql
initialize_specialization_keystones(in_keystones TEXT[]) RETURNS TABLE(keystone_id, keystone_name)
set_specialization_xp_and_level(in_player_id, in_track_type, in_xp_amount, in_level)
purchase_specialization_keystone(in_player_id, in_keystone) RETURNS BOOLEAN
update_specialization_refund_id(in_player_id, in_refund_id)
get_all_specialization_data(in_player_id) RETURNS SpecializationInfo
```

### Key Queries (dune-admin uses these)

```sql
-- All spec tracks
SELECT player_id, track_type::text, xp_amount, level
FROM dune.specialization_tracks ORDER BY player_id, track_type;

-- Award XP
UPDATE dune.specialization_tracks
SET xp_amount = xp_amount + $delta
WHERE player_id = $player_id AND track_type::text = $track_type;
-- (INSERT if no rows affected)

-- Reset all tracks for player
DELETE FROM dune.specialization_tracks WHERE player_id = $player_id;
DELETE FROM dune.purchased_specialization_keystones WHERE player_id = $player_id;
```

---

## 10. Journey / Quest System

**Files:** `journey_system.sql`, `79_proc_journey_updates.sql`

### Tables

| Table | Description |
|-------|-------------|
| `journey_story_node` | account_id, story_node_id, complete_condition_state (JSONB), reveal_condition_state, fail_condition_state, metadata_state, reset_group, override_reward_block, has_pending_reward |
| `journey_story_node_cooldown` | story_node_id, account_id, cooldown_end_time |

### Functions

```sql
save_journey_story_nodes(in_account_id, in_journey_data SaveJourneyData[])
save_journey_story_node(in_account_id, in_story_node_id, ...)  -- single-node upsert
update_journey_story_ids(old_story_ids TEXT[], new_story_ids TEXT[])  -- rename nodes after patch
delete_journey_story_ids(story_ids TEXT[])
complete_journey_nodes_where_prerequisite_nodes_are_complete(story_ids_to_complete, prerequisite_completed_story_ids)
```

---

## 11. Map, Markers & Overmap

**Files:** `44_proc_load_map_areas_entries.sql`, `14_proc_add_map_areas_time.sql`, `map_markers.sql`, `66_overmap_persistence.sql`, `31_proc_clear_map_areas_data_for_player.sql`

### Tables

| Table | Description |
|-------|-------------|
| `map_names` | map_name_id (SMALLINT), map_name TEXT |
| `markers` | marker_hash_id, dimension_index, marker (MARKER type), area_id, area_radius, long_range, payload (JSONB), map_name_id |
| `player_markers` | player_id, marker_hash_id, dimension_index, discovery_level, discovery_method, payload (JSONB), map_name_id |
| `map_areas` | account_id, area_id, time_discovered, time_first_entered, map_name, survey data |
| `overmap_players` | player_id, vehicle_id, has_polar_psu, overmap_location (Vector) |

### Functions

```sql
-- Map area tracking
add_map_areas_time_discovered(in_account_id, in_area_id, in_time_discovered, in_map_name)
add_map_areas_time_first_entered(in_account_id, in_area_id, in_time_first_entered, in_map_name)
add_map_areas_surveyed_items(in_account_id, in_area_id, ...)
load_map_areas_entries(in_account_id, in_map_name) RETURNS TABLE(area_id, time_discovered, time_first_entered, ...)
clear_map_areas_data_for_player(in_player_id, in_map_name)

-- Map markers
save_markers(in_player_marker_data SavePlayerMarkerData[], in_marker_data SaveMarkerData[])

-- Overmap (open desert survival layer)
overmap_save_player_survival_data(in_player_id, in_vehicle_id, in_has_polar_psu, in_overmap_location)
overmap_load_player_survival_data(in_player_id) RETURNS TABLE(vehicle_id, has_polar_psu, overmap_location)
overmap_delete_player_survival_data(in_player_id)
```

---

## 12. Spice Fields & Resource Fields

**Files:** `91_spicefield_system.sql`, `75_resource_field_state.sql`

### Tables

| Table | Description |
|-------|-------------|
| `spicefield_types` | type_id, max_globally_active/primed, current_globally_active/primed, is_spawning_active, global_spawn_weight, field_type, map_name, dimension_index |
| `spicefield_server_availability` | server_id, spicefield_type_id, inactive_fields_of_type, requested_spawned_of_type |
| `resourcefield_state` | map, dimension_index, field_kind_id, field_id, spawn_time, value_remaining |

### Functions (Spice Fields)

```sql
upsert_spicefield_types(in_max_globally_active int4[], in_max_globally_primed int4[], in_field_types text[], in_map_name, in_dimension_index)
update_spice_field_spawn_state(in_is_spawning_active BOOL, in_spicefield_type_id INT)
update_global_spice_field_rules(in_max_globally_primed, in_max_globally_active, in_spicefield_type_id)
fetch_spicefie_id_types_with_global_info(in_map_name, in_dimension_index) RETURNS TABLE(...)
fetch_server_spice_field_manifest(in_server_id TEXT) RETURNS TABLE(spicefield_type_id, inactive_fields_of_type, requested_spawned_of_type)
try_spawn_spicefield(in_source_server_id, in_spicefield_id) RETURNS BOOLEAN
try_prime_spicefield(in_source_server_id, in_spicefield_id) RETURNS BOOLEAN
try_restart_spicefield(in_server_id, in_spicefield_type_id) RETURNS BOOLEAN
request_spawn_spice_field(in_server_id, in_spicefield_type_id)
record_deactivated_spice_field(in_server_id, in_spicefield_type_id)
record_unreadied_spice_fields(in_server_id, in_spicefield_type_id, in_num_unreadied)
register_spice_field_server_resources(in_server_id, in_spicefield_type_ids INT[], in_inactive_fields_of_types INT[])
produce_spicefield_manifest(in_map_name, in_dimension_index) RETURNS TABLE(server, type_id, inactive_fields, requested_fields)
reset_global_spice_field_state(in_map_name, in_dimension_index)
```

### Functions (Resource Fields)

```sql
update_resourcefield_states(in_map, in_dimension_index, in_field_kind_id, in_field_states ResourceFieldStateEntry[])
remove_resourcefield_states(in_map, in_dimension_index, in_field_ids BIGINT[])
fetch_resourcefield_state(in_map, in_dimension_index, in_field_kind_id) RETURNS TABLE(field_id, spawn_time, value_remaining)
```

---

## 13. Totems & Landclaim

**Files:** `79_totems.sql`

### Tables

| Table | Description |
|-------|-------------|
| `totems` | id (FK actors), landclaim_vertical_level, last_backup_timestamp, landclaim_original_global_location (REAL[3]), landclaim_original_global_yaw_rotation |
| `landclaim_segments` | totem_id, grid_location_x, grid_location_y |

### Functions

```sql
get_landclaim_segments(in_totem_id BIGINT) RETURNS TABLE(grid_location_x, grid_location_y)
add_landclaim_segment(in_totem_id, in_grid_location_x, in_grid_location_y)
save_totem(in_id BIGINT, in_data TotemSaveData)
load_totem(in_id BIGINT) RETURNS TotemSaveData
```

---

## 14. Vendors

**File:** `76_vendor_stock.sql`

### Tables

| Table | Description |
|-------|-------------|
| `vendor_stock_cycle` | vendor_id, player_id, last_interacted_timestamp |
| `vendor_stock_state` | vendor_id, player_id, template_id, amount_bought |

### Functions

```sql
update_vendor_timestamp_for_player(in_vendor_id, in_player_id, in_timestamp)
player_purchased_item_from_vendor(in_vendor_id, in_player_id, in_template_id, in_amount_bought)
interact_get_vendor_items_bought_from_player(in_vendor_id, in_player_id, in_current_cycle_start_timestamp) RETURNS TABLE(template_id, amount_bought)
clean_vendors_older_than_timestamp(in_reference_timestamp BIGINT)
clean_stock_for_player(in_player_id BIGINT)
clean_stock_for_vendors(in_vendor_ids TEXT[])
```

---

## 15. Communinet (In-game Chat)

**Files:** `81_communinet.sql`, `22_proc_remove_communinet_player_channel.sql`

### Tables

| Table | Description |
|-------|-------------|
| `communinet_player` | account_id, is_active, selected_channel_name |
| `communinet_player_channels` | account_id, channel_name, is_tuned |

### Functions

```sql
remove_communinet_player_channel(in_account_id BIGINT, in_channel_name TEXT)
-- Additional CRUD functions in 81_communinet.sql
```

---

## 16. Mnemonic Recall (Codex)

**Files:** `85_mnemonic_recall.sql`, `25_proc_delete_mnemonic_recall_lesson.sql`, `26_proc_delete_mnemonic_recall_lesson_all.sql`

### Tables

| Table | Description |
|-------|-------------|
| `mnemonic_recall` | id, account_id, lesson_id (TEXT), lesson_state, lesson_progress, is_new |

### Functions

```sql
delete_mnemonic_recall_lesson(in_account_id BIGINT, in_lesson_id TEXT)
delete_mnemonic_recall_lesson_all(in_account_id BIGINT)
-- Load/save functions in 85_mnemonic_recall.sql
```

---

## 17. Friends & Player Info Lookup

**File:** `63_friends.sql`

### Functions

```sql
get_friends_search(in_player_name TEXT, in_max_players_count INTEGER)
RETURNS TABLE(player_id, character_name, funcom_id, platform_id, platform_name)
-- Uses pg_trgm SIMILARITY for fuzzy name matching
```

---

## 18. Dungeons

**File:** `dungeons_system.sql`

### Tables

| Table | Description |
|-------|-------------|
| `dungeon_completion` | completion_id, dungeon_id, difficulty, duration_ms, players_num |
| `dungeon_completion_players` | player_id, completion_id |

### Functions

```sql
record_dungeon_completion(in_dungeon_id, in_difficulty, in_duration_ms, players_ids BIGINT[])
get_best_dungeon_completion(in_dungeon_id TEXT) RETURNS TABLE(difficulty, duration_ms, players_names TEXT[])
get_best_dungeons_completions_for_player(in_player_id BIGINT) RETURNS TABLE(dungeon_id, difficulty, duration_ms, players_num)
delete_all_dungeon_completions(in_dungeon_id TEXT)
```

---

## 19. Vehicle Recovery

**File:** `vehicle_recovery.sql`

(See [§7 Vehicles](#7-vehicles) for the full table and function listing.)

---

## 20. Tutorials

**File:** `65_proc_tutorials.sql`

### Tables

| Table | Description |
|-------|-------------|
| `tutorials` | id (SMALLINT), name TEXT |
| `tutorial_per_player` | player_id, tutorial_id, tutorial_state (SMALLINT) |

### Functions

```sql
register_new_tutorials(tutorials TEXT[]) RETURNS TABLE(tutorial_id, tutorial_name)
create_or_update_tutorial_entry(in_player_id, in_tutorial_id, in_tutorial_state)
delete_all_tutorial_entries(in_player_id BIGINT)
get_all_tutorial_entries(in_player_id BIGINT) RETURNS TABLE(tutorial_id, tutorial_state)
```

---

## 21. Lore Pickups

**File:** `58_per_player_lore_pickup.sql`

### Tables

| Table | Description |
|-------|-------------|
| `lore_pickups` | incremental_id (SMALLINT), lore_pickup_id TEXT |
| `lore_pickups_temporary` | Same structure for transient lore |
| `per_player_lore_pickup` | player_id, incremental_id — which lore this player has collected |
| `per_player_lore_pickup_temporary` | Same for temporary lore |

### Functions

```sql
register_lore_pickup(in_lore_pickup_ids TEXT[]) RETURNS SETOF SMALLINT
register_temporary_lore_pickup(in_lore_pickup_ids TEXT[]) RETURNS SETOF SMALLINT
-- Per-player save/load functions also in file
```

---

## 22. Event Log (Game Events)

**File:** `70_events_log.sql`

### Tables

| Table | Description |
|-------|-------------|
| `game_events` | actor_id, universe_time (TIMESTAMP), map, partition_id, event_type (INT), x/y/z, custom_data (JSONB), player_facing_event |

### Functions

```sql
wipe_old_events_log(in_days_limit INTEGER)
add_event_log_data(in_game_event_owner, in_universe_time, in_map_name, in_partition_id, in_event_type, in_x/y/z, in_is_player_facing, in_custom_data)
add_event_log_data_batched(in_data EventLogBulkEntryData[])
load_events_log_data_from_player(in_actor_id, in_limit_entries_num) RETURNS TABLE(game_event_owner, universe_time, map_name, partition_id, event_type, x/y/z, custom_data)
```

### Key Query

```sql
-- Recent events for a player (player-facing only, newest first, then reversed)
SELECT actor_id, universe_time, map, event_type, x, y, z, custom_data
FROM game_events
WHERE actor_id = $player_id AND player_facing_event = true
ORDER BY universe_time DESC
LIMIT $n;
```

---

## 23. Event Logging (Solaris Audit)

**File:** `event_logging.sql`

### Tables

| Table | Description |
|-------|-------------|
| `event_log` | Partitioned UNLOGGED table: partition_id, category, message, function_name, event_time, meta (JSONB) |
| `event_log_default` | Default catch-all partition |
| Per-partition tables | Created automatically via trigger on `world_partition` insert |

### Enums

| Enum | Values |
|------|--------|
| `LogCategoryType` | `solaris` |
| `LogMessageType` | `update_solaris` |
| `LogFunctionType` | `adjust_player_solaris_balance`, `adjust_player_virtual_currency_balance`, `dune_exchange_modify_user_solari_balance`, `dune_exchange_retrieve_solaris_from_item` |

### Functions

```sql
create_event_log_partition_table(table_name TEXT, partition_id BIGINT)  -- PROCEDURE
create_event_log_partition() RETURNS TRIGGER   -- fired on world_partition INSERT
log_event_solaris(in_function_oid, in_message, in_controller_id, in_solaris_balance, in_solaris_delta)
```

---

## 24. Sinkcharts

**File:** `56_proc_sinkcharts.sql`

### Tables

| Table | Description |
|-------|-------------|
| `sinkcharts` | item_id (FK items), marker_hash_ids BIGINT[] — cartography item marker contents |

### Functions

```sql
create_sinkchart_for_map_area_id(in_item_id, in_creator_id, in_map_name, in_area_id) RETURNS INT
-- Load/resolve sinkcharts functions also in file
```

---

## 25. Building Blueprints & Favorites

(See [§6 Buildings](#6-buildings--placeables) for full details.)

---

## 26. Player Access Codes

**File:** `93_player_access_codes.sql`

### Functions

```sql
get_player_access_codes(in_account_id BIGINT) RETURNS TABLE(access_code INT, access_code_type INT)
create_server_player_access_codes(in_account_id, in_access_code, in_access_code_type, in_is_resettable)
delete_server_player_access_codes(in_account_id, in_access_code, in_access_code_type)
reset_server_all_player_access_codes(in_account_id BIGINT)  -- deletes resettable codes only
```

---

## 27. Admin & Account Tools

**Files:** `admin_tool.sql`, `82_account_takeover.sql`

### Functions

```sql
-- Offline player teleport
admin_move_offline_player(in_fls_id, in_target_partition_name, in_target_location Vector)
admin_move_offline_player_to_partition(in_fls_id, in_target_partition_id, in_target_location Vector)
is_player_offline(in_fls_id TEXT) RETURNS BOOLEAN

-- Account takeover (play as another player's character)
load_takeoverable_user_ids() RETURNS SETOF TakeoverCharacterDataComposite
set_account_as_takeoverable(in_user_id TEXT, in_new_user_id TEXT)
can_takeover_account(in_user_id TEXT) RETURNS BOOLEAN
takeover_account(in_user_to_takeover TEXT, in_current_user TEXT)
```

### Key Admin SQL Snippets

```sql
-- Teleport offline player to partition by label
SELECT admin_move_offline_player($fls_id, 'PartitionLabel', ROW(x,y,z)::Vector);

-- List available partition labels
SELECT label, map, dimension_index FROM dune.world_partition WHERE label IS NOT NULL ORDER BY label;
```

---

## 28. Cheat Detection

**File:** `cheat_detection_tracking.sql`

### Tables

| Table | Description |
|-------|-------------|
| `cheater_tracking` | id, event_time, fls_id TEXT, cheat_type (enum) |

### Cheat Types

`item_dup_on_store_vbt`, `item_dup_on_restore_vbt`, `exchange_order_dupe`, `exchange_negative_solaris`,
`negative_solaris`, `item_dup_on_suicide_after_travel`, `forced_respawn`, `died_in_no_dying_map`,
`item_action_while_persistence_off`, `player_died`, `player_undermeshed`

### Functions

```sql
log_cheating(in_fls_id TEXT, in_cheat_type cheat_type_enum, in_event_time TIMESTAMPTZ)
flag_player_as_cheater(...)  -- additional fields for severity etc.
```

### Key Query

```sql
-- Recent cheat events
SELECT fls_id, cheat_type, event_time FROM dune.cheater_tracking
WHERE event_time > NOW() - INTERVAL '7 days'
ORDER BY event_time DESC LIMIT 100;
```

---

## 29. Character Transfers

**File:** `character_transfers.sql`

Implements cross-battlegroup character migration. Exports and imports actor IDs, inventories, items, FGL entities, building blueprints, player state, faction data, specialization tracks, building favorites, communinet channels, and more.

Key function: `_character_transfer_get_patches_checksum()` — ensures source and target battlegroups are on the same schema version before allowing a transfer.

Entry kinds: `acc`, `act`, `inv`, `itm`, `fgl`, `bbp` (primary/ID-mapped); `Character`, `PlayerMarker`, `PlayerFaction`, `SpecializationTracks`, `BuildingFavorite`, etc. (secondary).

---

## 30. Demo / Trial Accounts

**File:** `demo_trial.sql`

### Tables

| Table | Description |
|-------|-------------|
| `demo_users` | fls_id TEXT PK, demo_playtime_seconds INT, demo_state (DemoState enum) |

### Functions

```sql
save_demo_account_time(in_fls_id, in_demo_playtime_seconds INT)
set_demo_state(in_user_id TEXT, in_demo_state DemoState)
get_all_demo_players() RETURNS TABLE(fls_ids TEXT)
-- Also: get_demo_player_data(), migrate_demo_to_retail(), etc.
```

---

## 31. Coriolis Storm

**File:** `70_coriolis.sql`

Manages the periodic mega-storm system that clears the open desert.

### Key type: `CoriolisMapInfo`

```sql
CREATE TYPE CoriolisMapInfo AS (
    is_affected_by_coriolis BOOLEAN,
    is_outside_shieldwall BOOLEAN,
    marker_types_to_keep TEXT[],
    should_clear_surveyed_areas BOOLEAN,
    vehicle_classes_spawned_on_map TEXT[]
);
```

Functions store and retrieve per-map Coriolis configuration and handle the periodic wipe of markers, surveyed areas, and exposed vehicles.

---

## 32. Encounters & Spawners

**Files:** `87_encounters.sql`, `80_spawners.sql`

### Tables

| Table | Description |
|-------|-------------|
| `encounters_static` | map_name, package_name, actor_name, encounter_name, waiting_for_reset |
| `actor_spawners` | id, map, name, dimension_index |
| `actor_spawner_actors` | spawner_id, actor_id |

---

## 33. World Partition Presets

**File:** `49_world_partition_presets.sql`

Stores preset partition layouts that can be applied when setting up a new server configuration.

---

## 34. Debug Utilities

**File:** `77_debug.sql`

```sql
debug_raise_notices(in_notices TEXT[])     -- RAISE NOTICE for each entry
debug_raise_exception(in_exception TEXT, in_notices TEXT[])
debug_echo(in_text TEXT, in_notices TEXT[]) RETURNS TEXT

-- Test table (used in server integration tests)
debug_test_table(entry TEXT)
debug_reset_test_table()
debug_add_test_table_data(in_entry TEXT)
debug_collect_test_table_data() RETURNS SETOF TEXT
```

---

## 35. Upgrade Migrations

**Directory:** `Upgrade/`

138+ numbered migration scripts (`01.sql` … `138.sql`) apply schema changes incrementally. Applied patches are tracked in the `applied_patches` table. The `get_applied_patches()` function returns them in order.

The `__order.txt` file defines the sequence when multiple upgrades are applied together.

---

## Quick Reference: Tables Used by dune-admin

| Feature | Tables |
|---------|--------|
| Player list | `actors`, `player_state`, `encrypted_accounts`, `player_faction` |
| Inventory | `items`, `inventories` |
| Currency | `player_virtual_currency_balances` |
| Faction rep | `player_faction_reputation`, `factions`, `player_tags` |
| Specializations | `specialization_tracks`, `purchased_specialization_keystones` |
| Online state | `player_state` (online_status, last_avatar_activity) |
| Buildings count | `buildings`, `totems`, `actors` |
| DB browser | `pg_stat_user_tables`, `information_schema.columns` |

## Quick Reference: Tables Used by market-bot

| Feature | Tables |
|---------|--------|
| Exchange detection | `dune_exchange_orders`, `dune_exchanges` |
| Bot inventory | `inventories`, `items` |
| Create listing | `items`, `dune_exchange_orders`, `dune_exchange_sell_orders` |
| Buy player listing | `dune_exchange_orders`, `dune_exchange_sell_orders`, `dune_exchange_fulfilled_orders`, `items` |
| Price stats | `dune_exchange_orders`, `dune_exchange_sell_orders`, `dune_exchange_fulfilled_orders` |
| Category cache | `dune_exchange_orders` (live scan) + SQLite local cache |
| Bot actor | `actors` (class = 'Revy') |
| Bot balance | `dune_exchange_users` |
