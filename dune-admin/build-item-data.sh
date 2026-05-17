#!/usr/bin/env bash
set -euo pipefail

CDT_BASE_PATH="${1:-../systems/Items/CDT_BaseItems.json}"
OUT_PATH="${2:-item-data.json}"
SCHEMATICS_PATH="${3:-../systems/Items/BaseItems/DT_BaseItems_Schematics.json}"
RECIPES_PATH="${4:-../systems/Crafting/DT_ItemsCraftingRecipes.json}"
LOOT_TABLES_PATH="${5:-../systems/LootTables/Loot_DifficultyScaled}"
UPGRADES_PATH="${6:-../systems/Items/Upgrades}"

if [ ! -f "$CDT_BASE_PATH" ]; then
  echo "CDT_BaseItems.json not found: $CDT_BASE_PATH" >&2
  exit 1
fi

# --- Step 1: build item-data.json from CDT_BaseItems.json ---

echo "Parsing $CDT_BASE_PATH..."
python3 - << PYEOF
import json, sys

CDT_PATH = "$CDT_BASE_PATH"
OUT = "$OUT_PATH"

TAG_TO_CATEGORY = {
    # Garments — with body slot sub-paths
    "Items.Clothes.ScoutArmor.Head":   "items/garment/lightarmor/head",
    "Items.Clothes.ScoutArmor.Torso":  "items/garment/lightarmor/chest",
    "Items.Clothes.ScoutArmor.Legs":   "items/garment/lightarmor/legs",
    "Items.Clothes.ScoutArmor.Hands":  "items/garment/lightarmor/hands",
    "Items.Clothes.ScoutArmor.Feet":   "items/garment/lightarmor/feet",
    "Items.Clothes.ScoutArmor":        "items/garment/lightarmor",
    "Items.Clothes.HeavyArmor.Head":   "items/garment/heavyarmor/head",
    "Items.Clothes.HeavyArmor.Torso":  "items/garment/heavyarmor/chest",
    "Items.Clothes.HeavyArmor.Legs":   "items/garment/heavyarmor/legs",
    "Items.Clothes.HeavyArmor.Hands":  "items/garment/heavyarmor/hands",
    "Items.Clothes.HeavyArmor.Feet":   "items/garment/heavyarmor/feet",
    "Items.Clothes.HeavyArmor":        "items/garment/heavyarmor",
    "Items.Clothes.Stillsuit.Head":    "items/garment/stillsuits/head",
    "Items.Clothes.Stillsuit.Torso":   "items/garment/stillsuits/chest",
    "Items.Clothes.Stillsuit.Hands":   "items/garment/stillsuits/hands",
    "Items.Clothes.Stillsuit.Feet":    "items/garment/stillsuits/feet",
    "Items.Clothes.Stillsuit":         "items/garment/stillsuits",
    "Items.Clothes.Social.Torso":      "items/garment/socialwearables/chest",
    "Items.Clothes.Social.Legs":       "items/garment/socialwearables/legs",
    "Items.Clothes.Social.Hands":      "items/garment/socialwearables/hands",
    "Items.Clothes.Social.Feet":       "items/garment/socialwearables/feet",
    "Items.Clothes.Utility":           "items/garment/utilitywearables",
    # Augments
    "Items.Augment.Armor":             "items/augment/armor",
    "Items.Augment.Melee":             "items/augment/melee",
    "Items.Augment.Ranged":            "items/augment/ranged",
    "Items.Augment.Misc":              "items/augment/misc",
    # Melee weapons
    "Items.Holsters.MeleeWeapons.Knife":  "items/weapons/shortblades",
    "Items.Holsters.MeleeWeapons.Sword":  "items/weapons/longblades",
    # Ranged weapons
    "Items.Holsters.RangedWeapons.Light.Pistol":          "items/weapons/pistol",
    "Items.Holsters.RangedWeapons.Light.SMG":             "items/weapons/smg",
    "Items.Holsters.RangedWeapons.Light.Rifle.SpitDart":  "items/weapons/spitdart",
    "Items.Holsters.RangedWeapons.Light.Rifle.BattleRifle":"items/weapons/battlerifle",
    "Items.Holsters.RangedWeapons.Light.Shotgun":         "items/weapons/shotgun",
    "Items.Holsters.RangedWeapons.Heavy.Pistol":          "items/weapons/heavypistol",
    "Items.Holsters.RangedWeapons.Heavy.LMG":             "items/weapons/heavyrifle",
    "Items.Holsters.RangedWeapons.Heavy.Shotgun":         "items/weapons/heavyshotgun",
    "Items.Holsters.RangedWeapons.Heavy.RocketLauncher":  "items/weapons/missilelauncher",
    "Items.Holsters.RangedWeapons.Heavy.Flamethrower":    "items/weapons/flamethrower",
    "Items.Holsters.RangedWeapons.Heavy.Fireballer":      "items/weapons/fireballer",
    "Items.Holsters.RangedWeapons.Heavy.Lasgun":          "items/weapons/lasgun",
    # Vehicles — base parts (VehicleBase.*)
    "Items.Holsters.Deployables.VehicleBase.Sandbike.Chassis":    "items/vehicles/sandbike/chassis",
    "Items.Holsters.Deployables.VehicleBase.Sandbike.Hull":       "items/vehicles/sandbike/hull",
    "Items.Holsters.Deployables.VehicleBase.Sandbike.Engine":     "items/vehicles/sandbike/engine",
    "Items.Holsters.Deployables.VehicleBase.Sandbike.PSU":        "items/vehicles/sandbike/psu",
    "Items.Holsters.Deployables.VehicleBase.Sandbike.Locomotion": "items/vehicles/sandbike/locomotion",
    "Items.Holsters.Deployables.VehicleBase.BuggyChoam.Chassis":    "items/vehicles/buggy/chassis",
    "Items.Holsters.Deployables.VehicleBase.BuggyChoam.Hull":       "items/vehicles/buggy/hull",
    "Items.Holsters.Deployables.VehicleBase.BuggyChoam.Rear":       "items/vehicles/buggy/rear",
    "Items.Holsters.Deployables.VehicleBase.BuggyChoam.Engine":     "items/vehicles/buggy/engine",
    "Items.Holsters.Deployables.VehicleBase.BuggyChoam.PSU":        "items/vehicles/buggy/psu",
    "Items.Holsters.Deployables.VehicleBase.BuggyChoam.Locomotion": "items/vehicles/buggy/locomotion",
    "Items.Holsters.Deployables.VehicleBase.LightOrniCHOAM.Chassis":    "items/vehicles/lightornithopter/chassis",
    "Items.Holsters.Deployables.VehicleBase.LightOrniCHOAM.Cockpit":    "items/vehicles/lightornithopter/cockpit",
    "Items.Holsters.Deployables.VehicleBase.LightOrniCHOAM.Hull":       "items/vehicles/lightornithopter/hull",
    "Items.Holsters.Deployables.VehicleBase.LightOrniCHOAM.Engine":     "items/vehicles/lightornithopter/engine",
    "Items.Holsters.Deployables.VehicleBase.LightOrniCHOAM.PSU":        "items/vehicles/lightornithopter/psu",
    "Items.Holsters.Deployables.VehicleBase.LightOrniCHOAM.Wing":       "items/vehicles/lightornithopter/locomotion",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.Chassis":   "items/vehicles/mediumornithopter/chassis",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.Cabin":     "items/vehicles/mediumornithopter/cabin",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.Cockpit":   "items/vehicles/mediumornithopter/cockpit",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.Tail":      "items/vehicles/mediumornithopter/tail",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.Engine":    "items/vehicles/mediumornithopter/engine",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.PSU":       "items/vehicles/mediumornithopter/psu",
    "Items.Holsters.Deployables.VehicleBase.MediumOrniCHOAM.Wing":      "items/vehicles/mediumornithopter/locomotion",
    "Items.Holsters.Deployables.VehicleBase.TransportOrniCHOAM.Chassis": "items/vehicles/transportornithopter/chassis",
    "Items.Holsters.Deployables.VehicleBase.TransportOrniCHOAM.Hull":    "items/vehicles/transportornithopter/hull",
    "Items.Holsters.Deployables.VehicleBase.TransportOrniCHOAM.Engine":  "items/vehicles/transportornithopter/engine",
    "Items.Holsters.Deployables.VehicleBase.TransportOrniCHOAM.PSU":     "items/vehicles/transportornithopter/psu",
    "Items.Holsters.Deployables.VehicleBase.TransportOrniCHOAM.Wing":    "items/vehicles/transportornithopter/locomotion",
    "Items.Holsters.Deployables.VehicleBase.SandcrawlerCHOAM.Chassis":  "items/vehicles/sandcrawler/chassis",
    "Items.Holsters.Deployables.VehicleBase.SandcrawlerCHOAM.Cabin":    "items/vehicles/sandcrawler/cabin",
    "Items.Holsters.Deployables.VehicleBase.SandcrawlerCHOAM.Engine":   "items/vehicles/sandcrawler/engine",
    "Items.Holsters.Deployables.VehicleBase.SandcrawlerCHOAM.PSU":      "items/vehicles/sandcrawler/psu",
    "Items.Holsters.Deployables.VehicleBase.SandcrawlerCHOAM.Locomotion":"items/vehicles/sandcrawler/locomotion",
    "Items.Holsters.Deployables.VehicleBase.Treadwheel.Chassis":    "items/vehicles/sandcrawler/chassis",
    "Items.Holsters.Deployables.VehicleBase.Treadwheel.Engine":     "items/vehicles/sandcrawler/engine",
    "Items.Holsters.Deployables.VehicleBase.Treadwheel.PSU":        "items/vehicles/sandcrawler/psu",
    "Items.Holsters.Deployables.VehicleBase.Treadwheel.Locomotion": "items/vehicles/sandcrawler/locomotion",
    # Vehicle extras (accessories/utility slots)
    "Items.Holsters.Deployables.VehicleExtra.Sandbike":             "items/vehicles/sandbike/utility",
    "Items.Holsters.Deployables.VehicleExtra.BuggyChoam":           "items/vehicles/buggy/utility",
    "Items.Holsters.Deployables.VehicleExtra.LightOrniCHOAM":       "items/vehicles/lightornithopter/utility",
    "Items.Holsters.Deployables.VehicleExtra.MediumOrniCHOAM":      "items/vehicles/mediumornithopter/utility",
    "Items.Holsters.Deployables.VehicleExtra.TransportOrniCHOAM":   "items/vehicles/transportornithopter/utility",
    "Items.Holsters.Deployables.VehicleExtra.Treadwheel":           "items/vehicles/sandcrawler/utility",
    # Utility tools
    "Items.Holsters.UtilityTools.Thumper":     "items/utility/deployables",
    "Items.Holsters.HydrationTools.Water":     "items/utility/hydrationtools/watertools",
    "Items.Holsters.HydrationTools.Blood":     "items/utility/hydrationtools/bloodtools",
    "Items.Holsters.GatheringTools.Cutteray":  "items/utility/gatheringtools/cutteray",
    "Items.Holsters.GatheringTools.Compactor": "items/utility/gatheringtools/compactor",
    "Items.Holsters.CartographyTools":         "items/utility/cartographytools",
    "Items.Holsters.UtilityTools.Shield":      "items/utility/utilitytools/shield",
    "Items.Holsters.UtilityTools.Suspensor":   "items/utility/utilitytools/suspensor",
    "Items.Holsters.UtilityTools.Power":       "items/utility/utilitytools/powerpack",
    # Resources / misc
    "Items.RawResources.Fuel":         "items/misc/fuel",
    "Items.RawResources":              "items/misc/rawresources",
    "Items.RefinedResources.Fuel":     "items/misc/fuel",
    "Items.RefinedResources":          "items/misc/refinedresources",
    "Items.CraftedResources":          "items/misc/components",
    "Items.Consumables.Health":        "items/utility/consumables/utility",
    "Items.Consumables.Spice":         "items/utility/consumables/utility",
    # Ammunition
    "Items.Holsters.RangedWeapons":    "items/weapons/ammunition",
}

# Sort keys longest-first for best-match lookup
SORTED_KEYS = sorted(TAG_TO_CATEGORY.keys(), key=len, reverse=True)

def get_category(tags):
    """Return the most specific (longest prefix) matching category for the item's tags."""
    best_key = None
    best_cat = None
    for tag in tags:
        for key in SORTED_KEYS:
            # Match if tag equals key OR tag starts with key + "."
            if tag == key or tag.startswith(key + "."):
                if best_key is None or len(key) > len(best_key):
                    best_key = key
                    best_cat = TAG_TO_CATEGORY[key]
                break  # found longest key for this tag; try next tag
    return best_cat

def get_rarity(tags):
    for t in tags:
        if t.startswith("Rarity."):
            return t[len("Rarity."):].lower()
    return None

def get_tier(tags):
    for t in tags:
        if t.startswith("LootTier."):
            try:
                return int(t[len("LootTier."):])
            except ValueError:
                pass
    return None

with open(CDT_PATH, 'rb') as f:
    data = json.loads(f.read().decode('utf-8-sig'))

rows = data[0]['Rows']
items = {}
skipped_disabled = 0
skipped_no_category = 0

for key, entry in rows.items():
    sd = entry.get('StaticData', {})
    sad = entry.get('StackAndDurability', {})

    # Skip disabled items
    if sd.get('bIsEnabled') is False:
        skipped_disabled += 1
        continue

    tags = sd.get('ItemTags') or []
    category = get_category(tags)

    # Skip items with no matching category
    if not category:
        skipped_no_category += 1
        continue

    name = sd.get('Name', {}).get('LocalizedString') or key
    stack_max = sad.get('MaxStackSize') or 1
    volume = sd.get('ItemSize') or 0
    vendor_price = sd.get('BaseBuyFromVendorPrice') or 0
    rarity = get_rarity(tags)
    tier = get_tier(tags)
    tradeable = not (
        'Items.ExcludeFromExchange' in tags or
        'Items.ActorBoundItem' in tags
    )

    item = {
        'name': name,
        'stack_max': stack_max,
        'volume': volume,
        'vendor_price': vendor_price,
        'category': category,
        'tradeable': tradeable,
    }
    if tier is not None:
        item['tier'] = tier
    if rarity is not None:
        item['rarity'] = rarity

    items[key] = item

result = {'items': items}
with open(OUT, 'w') as f:
    json.dump(result, f, separators=(',', ':'))

print(f"Wrote {len(items)} items to {OUT} (skipped {skipped_disabled} disabled, {skipped_no_category} with no category)")
PYEOF

# --- Step 2: merge schematics ---

if [ ! -f "$SCHEMATICS_PATH" ]; then
  echo "Schematics file not found ($SCHEMATICS_PATH), skipping schematic merge."
  exit 0
fi

echo "Extracting schematics from $SCHEMATICS_PATH..."
python3 - << PYEOF
import json

SCHEMATICS_PATH = "$SCHEMATICS_PATH"
OUT = "$OUT_PATH"

SCHEMATIC_TAG_TO_CATEGORY = {
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
    "Items.Schematics.HydrationTools.Water":     "items/utility/hydrationtools/watertools",
    "Items.Schematics.HydrationTools.Blood":     "items/utility/hydrationtools/bloodtools",
    "Items.Schematics.GatheringTools.Cutteray":  "items/utility/gatheringtools/cutteray",
    "Items.Schematics.GatheringTools.Compactor": "items/utility/gatheringtools/compactor",
    "Items.Schematics.CartographyTools":          "items/utility/cartographytools",
    "Items.Schematics.UtilityTools.Shield":      "items/utility/utilitytools/shield",
    "Items.Schematics.UtilityTools.Suspensor":   "items/utility/utilitytools/suspensor",
    "Items.Schematics.UtilityTools.Power":        "items/utility/utilitytools/powerpack",
    "Items.Schematics.Augments.Armor":  "items/augment/armor",
    "Items.Schematics.Augments.Melee":  "items/augment/melee",
    "Items.Schematics.Augments.Ranged": "items/augment/ranged",
    "Items.Schematics.Augments.Misc":   "items/augment/misc",
}

SORTED_SCHEMATIC_KEYS = sorted(SCHEMATIC_TAG_TO_CATEGORY.keys(), key=len, reverse=True)

def get_schematic_category(tags):
    best_key = None
    best_cat = None
    for tag in tags:
        for key in SORTED_SCHEMATIC_KEYS:
            if tag == key or tag.startswith(key + "."):
                if best_key is None or len(key) > len(best_key):
                    best_key = key
                    best_cat = SCHEMATIC_TAG_TO_CATEGORY[key]
                break
    return best_cat

def get_tier(tags):
    for t in tags:
        if t.startswith("LootTier."):
            try:
                return int(t[len("LootTier."):])
            except ValueError:
                pass
    return 3

with open(SCHEMATICS_PATH, 'rb') as f:
    schematics_data = json.loads(f.read().decode('utf-8-sig'))

with open(OUT) as f:
    catalog = json.load(f)

rows = schematics_data[0]['Rows']
schematics = {}
for key, entry in rows.items():
    sd = entry.get('StaticData', {})
    tags = sd.get('ItemTags') or []
    if 'Items.ExcludeFromExchange' in tags:
        continue
    category = get_schematic_category(tags)
    if not category:
        continue
    name = sd.get('Name', {}).get('LocalizedString') or key
    tier = get_tier(tags)
    schematics[key] = {
        'name': name,
        'stack_max': 1,
        'volume': 0.1,
        'tier': tier,
        'rarity': 'Unique',
        'vendor_price': 0,
        'category': category,
        'tradeable': True,
        'is_schematic': True,
    }

catalog['items'].update(schematics)

with open(OUT, 'w') as f:
    json.dump(catalog, f, separators=(',', ':'))

print(f"Merged {len(schematics)} schematics into {OUT}")
PYEOF

# --- Step 3: material costs from crafting recipes ---

if [ ! -f "$RECIPES_PATH" ]; then
  echo "Recipes file not found ($RECIPES_PATH), skipping material cost computation."
  exit 0
fi

echo "Computing material costs from $RECIPES_PATH..."
RECIPES_STRIPPED=$(mktemp)
sed 's/^\xef\xbb\xbf//' "$RECIPES_PATH" > "$RECIPES_STRIPPED"

python3 - << PYEOF
import json

with open("$RECIPES_STRIPPED") as f:
    recipes_data = json.load(f)

with open("$OUT_PATH") as f:
    catalog = json.load(f)

# Build vendor price lookup
vp = {k: v.get('vendor_price', 0) for k, v in catalog['items'].items()}

rows = recipes_data[0][0]['Rows'] if isinstance(recipes_data[0], list) else recipes_data[0]['Rows']

def cost_for_tier(ings, vp):
    total = 0
    for ing in ings:
        name = ing.get('Key', {}).get('Name', '')
        qty  = ing.get('Value', {}).get('Amount', 0)
        total += (vp.get(name) or 0) * qty
    return total

cost_by_item = {}  # item_key -> [cost0, cost1, cost2, cost3, cost4, cost5]
for key, val in rows.items():
    r = val.get('Recipe', {})
    out = (r.get('Outcome') or [{}])[0].get('Key', {}).get('Name', '')
    if not out:
        continue
    iqs = r.get('IngredientsPerQuality') or [{}]
    # Compute costs for each tier, padding to 6 with last tier
    tier_costs = []
    for i in range(6):
        tier = iqs[min(i, len(iqs)-1)]
        ings = tier.get('Ingredients', [])
        tier_costs.append(cost_for_tier(ings, vp))
    if any(c > 0 for c in tier_costs):
        cost_by_item[out] = tier_costs

count = 0
for key, entry in catalog['items'].items():
    if key in cost_by_item:
        costs = cost_by_item[key]
        if any(c > 0 for c in costs):
            entry['material_cost_per_grade'] = costs
            entry['material_cost'] = costs[-1]  # grade 5 (highest) for backward compat
            count += 1

with open("$OUT_PATH", 'w') as f:
    json.dump(catalog, f, separators=(',', ':'))

print(f"Added material_cost_per_grade to {count} items in $OUT_PATH")
PYEOF

rm "$RECIPES_STRIPPED"

# --- Step 4: is_gradeable from CDT_BaseItems item tags ---

echo "Computing is_gradeable from DifficultyScaled loot tables..."
python3 - << PYEOF
import json, glob, re, sys

LOOT_DIR = "$LOOT_TABLES_PATH"
OUT = "$OUT_PATH"

# Collect all schematics that appear in DifficultyScaled (overland testing station) loot tables.
# Ecolabs drop schematics at quality grades 1-5; the crafted item inherits that grade.
# An item is gradeable iff its <Name>_Schematic appears in one of these tables.
diff_schematics = set()
for fpath in glob.glob(f"{LOOT_DIR}/**/*.json", recursive=True):
    try:
        with open(fpath, 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
        for row in data[0].get('Rows', {}).values():
            name = row.get('ItemTemplateId', {}).get('Name', '')
            if name and name != 'None':
                diff_schematics.add(name)
    except:
        pass

def item_from_schematic(s):
    return re.sub(r'_Schematic$', '', s, flags=re.IGNORECASE)

with open(OUT) as f:
    catalog = json.load(f)

count = 0
for key, entry in catalog['items'].items():
    if entry.get('is_schematic'):
        # Schematics appear directly in DifficultyScaled tables under their own name.
        # A graded schematic (grade 1-5) lets the player craft the item at that grade.
        gradeable = key in diff_schematics
    else:
        # Non-schematic items: gradeable if their _Schematic variant drops from ecolabs.
        gradeable = f"{key}_Schematic" in diff_schematics
    catalog['items'][key]['is_gradeable'] = gradeable
    if gradeable:
        count += 1

with open(OUT, 'w') as f:
    json.dump(catalog, f, separators=(',', ':'))

print(f"Marked {count} items as gradeable (schematic present in DifficultyScaled loot tables)")
PYEOF

# --- Step 5: min_quality_level from augment upgrade JSON files ---

if [ ! -d "$UPGRADES_PATH" ]; then
  echo "Upgrades path not found ($UPGRADES_PATH), skipping min_quality_level computation."
  exit 0
fi

echo "Computing min_quality_level from augment upgrade files..."
python3 - << PYEOF
import json, glob, re

UPGRADES_DIR = "$UPGRADES_PATH"
OUT = "$OUT_PATH"

# Each DA_AUGMENT_<key>.json may have MinQualityLevel in its Properties.
# Strip the DA_AUGMENT_ prefix to get the item template key.
min_quality = {}
for fpath in glob.glob(f"{UPGRADES_DIR}/**/*.json", recursive=True):
    try:
        with open(fpath, 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
        for obj in data:
            name = obj.get('Name', '')
            props = obj.get('Properties', {})
            if 'MinQualityLevel' in props:
                item_key = re.sub(r'^DA_AUGMENT_', '', name)
                min_quality[item_key] = props['MinQualityLevel']
    except:
        pass

with open(OUT) as f:
    catalog = json.load(f)

count = 0
for key, mqv in min_quality.items():
    if key in catalog['items'] and mqv > 0:
        catalog['items'][key]['min_quality_level'] = mqv
        count += 1

with open(OUT, 'w') as f:
    json.dump(catalog, f, separators=(',', ':'))

print(f"Set min_quality_level on {count} augment items")
PYEOF
