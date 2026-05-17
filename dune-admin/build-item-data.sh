#!/usr/bin/env bash
set -euo pipefail

RAW_PATH="${1:-item-data-raw.json}"
OUT_PATH="${2:-item-data.json}"
NAMES_PATH="${3:-dune-item-names.json}"
SCHEMATICS_PATH="${4:-../systems/Items/BaseItems/DT_BaseItems_Schematics.json}"
RECIPES_PATH="${5:-../systems/Crafting/DT_ItemsCraftingRecipes.json}"
CDT_BASE_PATH="${6:-../systems/Items/CDT_BaseItems.json}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

if [ ! -f "$RAW_PATH" ]; then
  echo "Input file not found: $RAW_PATH" >&2
  echo "Run fetch-item-data.js in the browser on dune.gaming.tools to generate it." >&2
  exit 1
fi

# --- Step 1: build item-data.json from raw CDN data ---

ITEMS_JQ_FILTER='
  def extract($item; $s):
    (if ($s.categories | type) == "number"
     then ($item[$s.categories] | if type == "array"
           then [.[] | $item[.]] | map(select(type=="string")) |
                (if any(test("^items/misc/[^/]+$"))
                 then map(select(test("^items/misc/[^/]+$"))) | sort_by(length) | first
                 else sort_by(length) | last end)
           else null end)
     else null end) as $catPath |
    (if ($s.itemTags | type) == "number"
     then [$item[$s.itemTags][] | $item[.]]
     else [] end) as $tags |
    {
      name:         (if $s.name        | type == "number" then $item[$s.name]        else null end),
      stack_max:    (if $s.maxStackSize | type == "number" then $item[$s.maxStackSize] else 1   end // 1),
      volume:       (if $s.volume       | type == "number" then $item[$s.volume]       else 0   end // 0),
      tier:         (if $s.tier         | type == "number" then $item[$s.tier]         else null end),
      rarity:       (if $s.rarity       | type == "number" then $item[$s.rarity]       else null end),
      vendor_price: (if $s.baseBuyFromVendorPrice | type == "number" then $item[$s.baseBuyFromVendorPrice] else null end),
      category:     $catPath,
      tradeable:    ($tags | any(. == "Items.ExcludeFromExchange" or . == "Items.ActorBoundItem") | not)
    };
  {
    default_stack_max: 1,
    default_volume: 0,
    items: (
      [
        to_entries[] |
        .key as $lowKey |
        .value as $item |
        select($item | type == "array" and length > 1) |
        ($item[0]) as $s |
        select($s | type == "object") |
        ($nameMap[$lowKey] // $lowKey) as $properId |
        { key: $properId, value: extract($item; $s) }
      ] | from_entries
    )
  }
'

if [ -f "$NAMES_PATH" ]; then
  echo "Parsing $RAW_PATH (applying IDs from $NAMES_PATH)..."
  jq --slurpfile namesFile "$NAMES_PATH" \
    '($namesFile[0] | map({key: (.ID | ascii_downcase), value: .ID}) | from_entries) as $nameMap |'"$ITEMS_JQ_FILTER" \
    "$RAW_PATH" > "$OUT_PATH"
else
  echo "Parsing $RAW_PATH (no names file found, using raw keys)..."
  jq '{} as $nameMap |'"$ITEMS_JQ_FILTER" "$RAW_PATH" > "$OUT_PATH"
fi

echo "Wrote $OUT_PATH"

# --- Step 2: merge schematics ---

if [ ! -f "$SCHEMATICS_PATH" ]; then
  echo "Schematics file not found ($SCHEMATICS_PATH), skipping schematic merge."
  exit 0
fi

SCHEMATICS_JQ_FILTER='
  def tag_to_category: {
    "Items.Schematics.Clothes.ScoutArmor":  "items/garment/lightarmor",
    "Items.Schematics.Clothes.HeavyArmor":  "items/garment/heavyarmor",
    "Items.Schematics.Clothes.Stillsuit":   "items/garment/stillsuits",
    "Items.Schematics.Clothes.Utility":     "items/garment/utilitywearables",
    "Items.Schematics.MeleeWeapons.Knife":       "items/weapons/shortblades",
    "Items.Schematics.MeleeWeapons.DualDaggers": "items/weapons/shortblades",
    "Items.Schematics.MeleeWeapons.Sword":        "items/weapons/longblades",
    "Items.Schematics.RangedWeapons.Light.Pistol":            "items/weapons/pistol",
    "Items.Schematics.RangedWeapons.Light.Rifle.SMG":         "items/weapons/smg",
    "Items.Schematics.RangedWeapons.Light.Rifle.Spitdart":    "items/weapons/spitdart",
    "Items.Schematics.RangedWeapons.Light.Rifle.BattleRifle": "items/weapons/battlerifle",
    "Items.Schematics.RangedWeapons.Light.Shotgun":           "items/weapons/shotgun",
    "Items.Schematics.RangedWeapons.Heavy.Pistol":  "items/weapons/heavypistol",
    "Items.Schematics.RangedWeapons.Heavy.Rifle":   "items/weapons/heavyrifle",
    "Items.Schematics.RangedWeapons.Heavy.Shotgun": "items/weapons/heavyshotgun",
    "Items.Schematics.RangedWeapons.Exotic.MissileLauncher": "items/weapons/missilelauncher",
    "Items.Schematics.RangedWeapons.Exotic.Flamethrower":    "items/weapons/flamethrower",
    "Items.Schematics.RangedWeapons.Exotic.Fireballer":      "items/weapons/fireballer",
    "Items.Schematics.RangedWeapons.Exotic.Lasgun":          "items/weapons/lasgun",
    "Items.Schematics.Deployables.VehicleBase.Sandbike":           "items/vehicles/sandbike",
    "Items.Schematics.Deployables.VehicleBase.BuggyChoam":         "items/vehicles/buggy",
    "Items.Schematics.Deployables.VehicleBase.LightOrniCHOAM":     "items/vehicles/lightornithopter",
    "Items.Schematics.Deployables.VehicleBase.MediumOrniCHOAM":    "items/vehicles/mediumornithopter",
    "Items.Schematics.Deployables.VehicleBase.TransportOrniCHOAM": "items/vehicles/transportornithopter",
    "Items.Schematics.Deployables.VehicleBase.SandcrawlerCHOAM":   "items/vehicles/sandcrawler",
    "Items.Schematics.Deployables.VehicleExtra.Sandbike":           "items/vehicles/sandbike",
    "Items.Schematics.Deployables.VehicleExtra.BuggyChoam":         "items/vehicles/buggy",
    "Items.Schematics.Deployables.VehicleExtra.LightOrniCHOAM":     "items/vehicles/lightornithopter",
    "Items.Schematics.Deployables.VehicleExtra.LightOrniChoam":     "items/vehicles/lightornithopter",
    "Items.Schematics.Deployables.VehicleExtra.MediumOrniCHOAM":    "items/vehicles/mediumornithopter",
    "Items.Schematics.Deployables.VehicleExtra.TransportOrniCHOAM": "items/vehicles/transportornithopter",
    "Items.Schematics.Deployables.VehicleExtra.Treadwheel":         "items/vehicles/sandcrawler",
    "Items.Schematics.UtilityTools.Thumper":     "items/utility/deployables",
    "Items.Schematics.HydrationTools.Water":     "items/utility/watertools",
    "Items.Schematics.HydrationTools.Blood":     "items/utility/bloodtools",
    "Items.Schematics.GatheringTools.Cutteray":  "items/utility/cutteray",
    "Items.Schematics.GatheringTools.Compactor": "items/utility/staticcompactor",
    "Items.Schematics.CartographyTools":          "items/utility/cartographytools",
    "Items.Schematics.UtilityTools.Shield":      "items/utility/shield",
    "Items.Schematics.UtilityTools.Suspensor":   "items/utility/suspensor",
    "Items.Schematics.UtilityTools.Power":        "items/utility/powerpack",
    "Items.Schematics.Augments.Armor":  "items/augment/armor",
    "Items.Schematics.Augments.Melee":  "items/augment/melee",
    "Items.Schematics.Augments.Ranged": "items/augment/ranged",
    "Items.Schematics.Augments.Misc":   "items/augment/misc"
  };

  def get_tier(tags):
    (tags | map(select(startswith("LootTier.")))
          | if length > 0 then (.[0] | ltrimstr("LootTier.") | tonumber) else 3 end)
    // 3;

  .[0].Rows | to_entries | map(
    .key as $key |
    .value.StaticData as $sd |
    ($sd.ItemTags // []) as $tags |
    select(($tags | index("Items.ExcludeFromExchange")) == null) |
    ($tags | map(. as $t | tag_to_category | to_entries | map(select(.key == $t)) | .[0]?.value)
           | map(select(. != null)) | .[0]) as $cat |
    select($cat != null) |
    { key: $key, value: {
        name: ($sd.Name.LocalizedString // $key), stack_max: 1, volume: 0.1,
        tier: get_tier($tags), rarity: "Unique", vendor_price: 0,
        category: $cat, tradeable: true, is_schematic: true
    }}
  ) | from_entries
'

echo "Extracting schematics from $SCHEMATICS_PATH..."
STRIPPED=$(mktemp)
sed 's/^\xef\xbb\xbf//' "$SCHEMATICS_PATH" > "$STRIPPED"
SCHEMATICS_JSON=$(jq "$SCHEMATICS_JQ_FILTER" "$STRIPPED")
rm "$STRIPPED"

COUNT=$(echo "$SCHEMATICS_JSON" | jq 'length')
TMP=$(mktemp)
jq --argjson s "$SCHEMATICS_JSON" '.items = (.items + $s)' "$OUT_PATH" > "$TMP"
mv "$TMP" "$OUT_PATH"
echo "Merged $COUNT schematics into $OUT_PATH"

# --- Step 3: material costs from crafting recipes ---

if [ ! -f "$RECIPES_PATH" ]; then
  echo "Recipes file not found ($RECIPES_PATH), skipping material cost computation."
  exit 0
fi

MATERIAL_COST_JQ='
  # $recipes is DT_ItemsCraftingRecipes.json (already parsed, array with Rows dict).
  # For each recipe: use the LAST IngredientsPerQuality tier (highest grade = highest cost).
  # Compute material_cost = sum of (vendor_price * quantity) for each ingredient.
  # Merge into .items[output_item].material_cost.

  . as $items |
  (
    $recipes[0][0].Rows | to_entries | map(
      .value.Recipe as $r |
      ($r.Outcome // [])[0].Key.Name as $out |
      select($out != null and $out != "") |
      (($r.IngredientsPerQuality // [{}])[-1:][0].Ingredients // []) as $ings |
      {
        key: $out,
        value: (
          $ings | map(
            (.Key.Name as $n |
             ($items.items[$n].vendor_price // 0) * .Value.Amount)
          ) | add // 0
        )
      }
    ) | from_entries
  ) as $costs |
  .items |= with_entries(
    if $costs[.key] != null and $costs[.key] > 0
    then .value.material_cost = $costs[.key]
    else .
    end
  )
'

echo "Computing material costs from $RECIPES_PATH..."
RECIPES_STRIPPED=$(mktemp)
sed 's/^\xef\xbb\xbf//' "$RECIPES_PATH" > "$RECIPES_STRIPPED"
TMP=$(mktemp)
jq --slurpfile recipes "$RECIPES_STRIPPED" "$MATERIAL_COST_JQ" "$OUT_PATH" > "$TMP"
mv "$TMP" "$OUT_PATH"
rm "$RECIPES_STRIPPED"

COST_COUNT=$(jq '[.items | to_entries[] | select(.value.material_cost != null)] | length' "$OUT_PATH")
echo "Added material_cost to $COST_COUNT items in $OUT_PATH"

# --- Step 4: is_gradeable from CDT_BaseItems item tags ---

if [ ! -f "$CDT_BASE_PATH" ]; then
  echo "CDT_BaseItems not found ($CDT_BASE_PATH), skipping is_gradeable computation."
  exit 0
fi

echo "Computing is_gradeable from $CDT_BASE_PATH..."
python3 - << PYEOF
import json, sys

CDT_PATH = "$CDT_BASE_PATH"
OUT = "$OUT_PATH"

with open(CDT_PATH, 'rb') as f:
    cdt = json.loads(f.read().decode('utf-8-sig'))
rows = cdt[0]['Rows']

EXCLUDED_TAGS = {'Items.CraftedResources', 'Items.RawResources',
                 'Items.RefinedResources', 'Items.Schematics'}

gradeable_map = {}
for item_id, entry in rows.items():
    tags = set(entry.get('StaticData', {}).get('ItemTags', []) or [])
    # Gradeable = can drop from loot pools with quality grades.
    # Proxy: has any LootTier.* tag (present on all loot-eligible equipment).
    # Items without this tag are crafted-only or story-progression items (grade 0 only).
    has_loot_tier = any(t.startswith('LootTier.') for t in tags)
    excluded = bool(tags & EXCLUDED_TAGS)
    gradeable_map[item_id] = has_loot_tier and not excluded

with open(OUT) as f:
    data = json.load(f)

count = 0
for key in data['items']:
    val = gradeable_map.get(key, False)
    data['items'][key]['is_gradeable'] = val
    if val:
        count += 1

with open(OUT, 'w') as f:
    json.dump(data, f, separators=(',', ':'))

print(f"Marked {count} items as gradeable in {OUT}")
PYEOF
