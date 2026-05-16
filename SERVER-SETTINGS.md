# Dune Awakening Server Settings Reference

Edit via FileBrowser: http://192.168.0.72:18888

---

## PART 1: Official Settings (FileBrowser)

### UserEngine.ini (UserSettings folder)

```ini
[ConsoleVariables]
; Server display name (overrides sietch names - remove to use Abbir/al-Mut/etc)
Bgd.ServerDisplayName="My Server"

; Server password
Bgd.ServerLoginPassword="password"

; Mining multipliers
Dune.GlobalMiningOutputMultiplier=1.0
Dune.GlobalVehicleMiningOutputMultiplier=1.0
SecurityZones.PvpResourceMultiplier=2.5

; Vehicle durability (0-10, 0=off)
dw.VehicleDurabilityDamageMultiplier=1.0

; Sandstorm settings
Sandstorm.Enabled=1
Sandstorm.Treasure.Enabled=1

; Sandworm settings
sandworm.dune.Enabled=1
Vehicle.SandwormCollisionInteraction=false
Sandworm.SandwormDangerZonesEnabled=true
Vehicle.SandwormInvulnerabilitySecondsOnExit=900.0
Vehicle.SandwormInvulnerabilitySecondsOnServerRestart=7200.0
```

### UserGame.ini (UserSettings folder)

```ini
[/Script/DuneSandbox.PvpPveSettings]
; Force PVP everywhere
m_bShouldForceEnablePvpOnAllPartitions=False
; Enable PVP on specific partitions (sietch dimensions)
;+m_PvpEnabledPartitions=1
;+m_PvpEnabledPartitions=2

[/Script/DuneSandbox.SecurityZonesSubsystem]
; Disable security zones = PVP everywhere
m_bAreSecurityZonesEnabled=True

[/DeteriorationSystem.ItemDeteriorationConstants]
; Item decay rate (0=off)
UpdateRateInSeconds=1.0

[/Script/DuneSandbox.SandStormConfig]
; Coriolis storm (big dangerous one)
m_bCoriolisAutoSpawnEnabled=True

[/Script/DuneSandbox.BuildingSettings]
; Max landclaims per player
m_MaxNumLandclaimSegments=6
; Landclaim expansion limits
m_BuildingBlueprintMaxExtensions=4
m_BaseBackupMaxExtensions=8
; Building restrictions
m_bBuildingRestrictionLimitsEnabled=True
```

---

## PART 2: Hidden Settings (DefaultGame.ini)

These are extracted from `/home/dune/server/DuneSandbox/Config/DefaultGame.ini`.
Add to UserGame.ini to override defaults.

### Time of Day / Day-Night Cycle

```ini
[/Script/DuneSandbox.TimeOfDaySettings]
m_bTimeOfDayEnabled=True
m_DayLengthMinutes=30.000000           ; Real minutes per in-game day (default 30)
m_StartingExperienceTimeOfDay=8.000000 ; Starting hour (8 AM)
```

### Giant Sandworm (Shai-Hulud) Spawning

```ini
[/Script/DuneSandbox.SandwormSettings]
m_EnableSandwormSystem=UseAllowList
m_bGiantWormSystemEnabled=True
m_GiantWormSpawningUpdateFrequency=60.000000      ; Check every 60 seconds
m_GiantWormSpawningCooldown=7200.000000           ; 2 hours between giant worm spawns
m_GiantWormMinimumSpiceAmountHarvested=50000.000000  ; Spice harvested to trigger
m_GiantWormMinimumPlayersOnSpiceField=4           ; Players needed on field
m_GiantWormMinimumDistanceFromIgwBoundary=2000.000000
m_MinDistanceBetweenSandworms=80000.000000        ; Min distance between worms
m_bEnableDangerZones=True
m_bEnableHibernation=True
m_MaximumTargetHeight=5000.000000
m_MinimumTargetHeight=-2000.000000
m_bGenerateTerritoriesFromHeatMap=True
m_bEnableDebugInfoReplication=True
```

### Spice Harvesting System

```ini
[/Script/DuneSandbox.SpiceHarvestingSystem]
m_bSpawningActive=True
m_PrimeRateInSeconds=30.000000                    ; Spice field prime rate
m_ManagerTickRateInSeconds=5.000000
m_ManagerRequestRefreshRateInSeconds=90.000000
m_GlobalManagerRequestRefreshRateInSeconds=120.000000
m_bPlayerMustWitnessBloom=False
m_bEnableSpiceBloomLongRangeReplication=True
m_bEnableSpiceFieldLongRangeReplication=True
m_MaxWaitTimeForIgwQuery=2.0

; Per-map spice field limits (can customize)
; DeepDesert_1: Small=60, Medium=12, Large=1
; Survival_1: Small=5 only
; Default: Small=6/3, Medium=10/5, Large=5/3 (primed/active)
```

### Death, Loot & Respawn

```ini
[/Script/DuneSandbox.DuneGameSettings]
m_bShouldPlayersDropLootOnDeath=False             ; Drop loot on death
m_bShouldPlayersDropLootOnDefeat=True             ; Drop loot on defeat
m_bShouldPlayersLoseItemsOnDeath=True             ; Lose items on death
m_bShouldNpcDropLootOnDeath=True                  ; NPCs drop loot

m_PVPRespawn=(m_FallbackRespawnTimeMinutes=8.0)   ; PVP respawn time
m_DefaultRespawn=(m_FallbackRespawnTimeMinutes=8.0) ; Default respawn time
```

### Vehicle Settings

```ini
[/Script/DuneSandbox.DuneVehicleSettings]
m_LastDamageDealtTimeThreshold=1.000000

; Time for vehicle wreck to drop a loot blob (seconds)
m_TimeToDropWreckBlobPerVehicleMap=(
  Sandbike=7200.0,
  Sandcrawler=7200.0,
  LightOrnithopter=7200.0,
  HeavyOrnithopter=7200.0,
  Carryall=7200.0,
  Tank=7200.0,
  Seeker=7200.0,
  MediumOrnithopter=7200.0,
  TransportOrnithopter=7200.0,
  Buggy=7200.0
)

; Recovery cost multipliers per vehicle
m_RecoveryPerVehicleClassCurrencyMultipliers=(
  Sandbike=0.2,
  TreadWheel=0.2,
  Buggy=0.6,
  LightOrnithopter=1.0,
  MediumOrnithopter=1.2,
  TransportOrnithopter=2.0,
  SandCrawler=2.0
)
```

### Building System

```ini
[/Script/DuneSandbox.BuildingSettings]
m_bMitigateAllSandstormDamage=False               ; Buildings immune to sandstorm
m_bEnableStabilizationSystem=True                 ; Building stability system
m_bEnableDestabilizationSystem=False              ; Building collapse
m_bEnableStabilizationSystemVFX=False
m_bEnableBuildingDestructionEffects=True
m_FallbackDefaultBuildingHealth=2500.000000
m_FallbackDefaultPlaceableHealth=400.000000
m_DamageVisualizationMultiplier=1.000000

; Sand buildup
m_SandBuildUpPlaceablesShelteredTargetValue=0.300000
m_SandBuildUpPlaceablesUnShelteredTargetValue=0.700000

; Landclaim settings
m_MaxNumLandclaimSegments=6
m_BuildingBlueprintMaxExtensions=4
m_BaseBackupMaxExtensions=8
m_bBuildingRestrictionLimitsEnabled=True
m_bCanRemoveBuildablesWithNoOwner=True

; Access levels
m_MinAccessLevelRange=1
m_MaxAccessLevelRange=5
m_DefaultProfileAccessLevel=3
m_DefaultPlaceableAccessLevel=1

; Totem codes
m_MinTotemCodeRandomRange=100000
m_MaxTotemCodeRandomRange=999999

; Pentashield
m_PentashieldMaxDetectionDistance=3000.000000
m_PentashieldSurfaceMaxHeight=5568.000000
m_PentashieldSurfaceMaxWidth=7424.000000

; Refund/repair
m_DefaultRepairCostMultiplier=0.500000
; m_DefaultBuildingSystemModifiers=(m_RefundPercentage=1.0, m_PlacementCostMultiplier=1.0)

; Server border building
m_bEnableBuildingNearServerBorders=False
m_bMinBuildableDistanceFromServerBorder=1000.000000
; DeepDesert custom: 10000.0

; Free placement limits
m_FreeTranslateMax=100.000000
m_FreeRotateMax=45.000000
```

### Crafting System

```ini
[/Script/DuneSandbox.CraftingSettings]
m_MaximumNumberOfCharges=999
m_MaximumNumberOfRequestsPerRecipe=1000
m_CostCheatMaxRecipes=500
m_MaxArraySerializationSize=500

; Various recipe output multipliers exist for:
; - Ore Refining (IronBar, SteelBar, CopperBar, etc.) - 2x at level 1
; - Spice Refining - 1.15x to 1.75x based on level
; - Water/Ice Refining - 1.2x to 2.0x based on level
; - Consumables (Health, Repair, Ammo) - bonus multipliers
```

### Hydration & Biome

```ini
[/Script/DuneSandbox.CommuninetSettings]
m_bHydrationEnabled=True
m_BiomeTierUpdateRateSeconds=2.5

[/Script/DuneSandbox.BiomeSettings]
m_SandBuildupMultiplier=1.0
```

### Ping System

```ini
[/Script/DuneSandbox.PingSystemSettings]
m_PingsPerPlayerLimit=5
m_PingMaximumDistance=2000.000000
m_PingPlacementHeightOffset=0.500000
m_QuickPingDoubleTapActionDelayInSeconds=0.300000
m_PingInWorldMarkerExpiryTime=5
m_PingMapMarkerExpiryTime=60
```

### Shelter Detection

```ini
[/Script/DuneSandbox.ShelterSettings]
m_ShelterTraceLength=10000.000000
m_ShelterTraceGridCellSize=200.000000
m_ShelterTraceFrameBudget=350
m_BuildingShelterThreshold=0.9
m_PlaceableShelterThreshold=0.65
m_bUseShelterGeneralTraceOnly=false
```

### AI & NPC Combat

```ini
[/Script/DuneSandbox.DuneAISettings]
m_MaxReinforcementSize=150.000000
m_MinAttackDelayTime=0.200000

; AI combat threat decay
m_ThreatDecayPerSecond=0.100000
m_ThreatDecayCooldown=1.000000

; Melee/ranged engagement limits via curves
```

### Map Features

```ini
[/Script/DuneSandbox.MapFeatures]
; Per-map feature toggles:
; - m_Surveying (survey probe)
; - m_Taxation
; - m_FogOfWar
; - m_SocialOnly (no combat)
; - m_StoryGameplay
; - m_WildLife
; - m_DeepDesertGameplay
; - m_ShiftingSands
; - m_bIsInstanced
; - m_bIsOutdoors
; - m_bAllowCharacterTransfer
```

---

## PART 3: GM/Admin Commands

From `DedicatedServerGame.ini`:

```ini
[AdminSetting.Global]
; Basic allowed commands
+Allowed_Commands=obj
+Allowed_Commands=FGL.ComponentAuditRequested

; GM Commands
+Allowed_GM_Commands=AddItemToInventory
+Allowed_GM_Commands=AddBasicInventoryToCharacter
+Allowed_GM_Commands=SpawnVehicle
+Allowed_GM_Commands=PatrolShipTeleportToNearest
+Allowed_GM_Commands=TeleportTo
+Allowed_GM_Commands=TeleportToMap
+Allowed_GM_Commands=TeleportToExact
+Allowed_GM_Commands=TeleportToPlayer
+Allowed_GM_Commands=TeleportToVehicleSpawner
+Allowed_GM_Commands=TeleportToSandworm
+Allowed_GM_Commands=TeleportToPersonalMarker
+Allowed_GM_Commands=TravelTo
+Allowed_GM_Commands=TravelToDimension
+Allowed_GM_Commands=Fly
+Allowed_GM_Commands=Ghost
+Allowed_GM_Commands=Walk
+Allowed_GM_Commands=DestroyTargetVehicle
+Allowed_GM_Commands=DestroyTotem
+Allowed_GM_Commands=DestroyPlaceable
+Allowed_GM_Commands=DestroyEntireBuilding
+Allowed_GM_Commands=DestroyBuildingPiece
+Allowed_GM_Commands=PrintPos
```

---

## PART 4: Server Tick Rates

From `DefaultGame.ini` - Map FPS limits:

| Map | Server FPS |
|-----|------------|
| HaggaBasin | 20 |
| DeepDesert | 20 |
| HarkoVillage | 10 |
| Arrakeen | 10 |
| Overland | 10 |
| BeneathCarthag | 20 |
| WaterFat | 20 |
| WreckOfHephaestus | 20 |
| ProcesVerbal | 10 |
| Story/Dungeons | 20 |

---

## Sietch Names (by dimension index)

| Index | Name |
|-------|------|
| 0 | Abbir |
| 1 | al-Mut |
| 2 | Alraab |
| 3 | Barkan |
| 4 | Coanua |
| 5 | Eaqrab |
| 6 | Fajr |
| 7 | Gara Kulon |
| 8 | Hajar |
| 9 | Jacurutu |
| 10 | Kathib |
| 11 | Khafash |
| 12 | Legg |
| 13 | Makab |
| 14 | Nadir |
| 15 | Rajifiri |
| 16 | Ramal |
| 17 | Rifana |
| 18 | Saajid |
| 19 | Sandrat |
| 20 | Ta'lab |
| 21 | Tabr |
| 22 | Tharwa |
| 23 | Umbu |
| 24 | Yaracuwan |

---

## Memory Requirements

| Map | RAM |
|-----|-----|
| Survival_1 (sietch) | ~12GB each |
| DeepDesert_1 | ~15GB |
| Overmap | ~2GB |
| SH_Arrakeen | ~2GB |
| SH_HarkoVillage | ~2GB |
| Story/Dungeons | ~2-6GB each |

---

## Current Configuration

- 1 Sietch (Abbir)
- DeepDesert enabled
- Social Hubs (Arrakeen + HarkoVillage) enabled
- On-demand autoscaling for dungeons/story maps

---

## How to Apply Hidden Settings

1. SSH to VM: `ssh dune@192.168.0.72`
2. Edit UserGame.ini via FileBrowser (http://192.168.0.72:18888) or directly
3. Add sections from above with your customized values
4. Restart battlegroup: `/home/dune/.dune/bin/battlegroup restart`

**Note:** Not all settings may work - some are read-only at runtime. Test changes incrementally.
