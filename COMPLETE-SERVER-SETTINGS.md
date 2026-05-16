# Dune Awakening Server Settings - COMPLETE Reference
## Extracted from DefaultGame.ini (v1.4.0.0 / Build 1957345)

This is a comprehensive extraction from the actual server configuration. Settings prefixed with `+` are array values (Unreal Engine array append syntax).

---

## PART 1: Game Settings (UserGame.ini Overrides)

### [/Script/DuneSandbox.TimeOfDaySettings]
```ini
m_bTimeOfDayEnabled=True
m_DayLengthMinutes=30.000000              ; Real minutes per in-game day
m_StartTime=12.000000                      ; Initial time on server start
m_StartingExperienceTimeOfDay=8.000000    ; Starting hour for new characters (8 AM)
m_AzimuthInDegrees=55.000000
m_SunriseInDegrees=180.000000
m_CharacterCustomizationTimeOfDay=10.000000
m_AuroraProbability=25                    ; % chance of aurora at night
```

### [/Script/DuneSandbox.SandwormSettings]
```ini
m_EnableSandwormSystem=UseAllowList
+m_SpawningAllowedBaseMapList=(Name="HaggaBasin")
+m_SpawningAllowedBaseMapList=(Name="DeepDesert")
m_bGenerateTerritoriesFromHeatMap=True
m_MinDistanceBetweenSandworms=80000.000000
m_SummonHeight=-5000.000000
m_bEnableDebugInfoReplication=True
m_DebugInfoReplicationFrequency=5.000000
m_bEnableDangerZones=True
m_DangerZonesCooldown=1.000000
m_SyncTargetIntervalSeconds=1.000000
UpdateTargetTimeSeconds=1.000000
m_MaximumTargetHeight=5000.000000
m_MinimumTargetHeight=-2000.000000
m_UnstuckTimer=3.000000
m_UnstuckDistance=1500.000000
m_bEnableHibernation=True
m_DefaultRoamingElevation=-4000.000000

; Threat Generation
ThreatScale=1.000000
DefaultMaxThreatScore=5000.000000
MaxThreatInSafezone=0.000000
InitialThreatRate=0.000000
m_RealTargetPickupRange=60000.000000
EnableBuildingThreatGeneration=True
ThreatDecreasingValuePerSec=0.000000
AirborneThreatDecreasingValuePerSec=100.000000
WalkingThreatPerSec=15.000000
WWoRThreatPerSec=5.000000
RunningThreatPerSec=20.000000
SprintingThreatPerSec=20.000000
CrouchingThreatPerSec=15.000000
SuspendingThreatPerSec=200.000000
DashingThreatPerSec=90.000000
ShieldingThreatPerSec=500.000000
DrumsandThreatPerSec=200.000000
VehicleShieldingThreatPerSec=50.000000
HyperSprintingThreatPerSec=90.000000
ThreatDecreaseCooldownInSeconds=5.000000
PlayerShootingRecoilThreatFactor=1.000000
NPCShootingRecoilThreatFactor=1.650000
PlayerVehicleShootingThreatFactor=1.000000
NPCVehicleShootingThreatFactor=1.000000
m_SpiceBlobLifespan=420.000000
HarvestSpicePickupThreatUnit=10.000000
HarvestSpiceCoalesceThreatUnit=10.000000
HarvestFlourSandPickupThreatUnit=10.000000
HarvestFlourSandCoalesceThreatUnit=10.000000
m_ThreatBlobDurabilityOutsideHeightRequirements=15.000000

; Giant Worm (Shai-Hulud)
m_bGiantWormSystemEnabled=True
m_GiantWormSpawningUpdateFrequency=60.000000
m_GiantWormSpawningCooldown=7200.000000           ; 2 hours between spawns
m_GiantWormSafezoneDetectionDistance=35000.000000
m_GiantWormSequenceLifespan=78.000000
m_GiantWormSequencePlayDelay=30.000000
m_GiantWormSpiceFieldType=(Name="Large")
m_GiantWormMinimumSpiceAmountHarvested=50000.000000
m_GiantWormMinimumPlayersOnSpiceField=4
m_GiantWormMinimumDistanceFromIgwBoundary=2000.000000

; Thumper Enrage
m_TimeToStopEnrage=60.000000
m_TimeToStopThumperEnrage=60.000000
m_TimeToCountThumpersForThumperEnrage=300.000000
m_NumberOfThumpersEatenForThumperEnrage=3
```

### [/Script/DuneSandbox.SpiceHarvestingSystem]
```ini
m_bSpawningActive=True
m_PrimeRateInSeconds=30.000000
m_ManagerTickRateInSeconds=5.000000
m_ManagerRequestRefreshRateInSeconds=90.000000
m_GlobalManagerRequestRefreshRateInSeconds=120.000000
m_bPlayerMustWitnessBloom=False
m_bEnableSpiceBloomLongRangeReplication=True
m_bEnableSpiceFieldLongRangeReplication=True
m_NodeValueToSpiceResourceRatio=10.000000

; Per-map spice field limits (Small/Medium/Large)
; DeepDesert_1: Small=60, Medium=12, Large=1
; Survival_1: Small=5 only
; Default: Small=6/3, Medium=10/5, Large=5/3 (primed/active)
```

### [/Script/DuneSandbox.BuildingSettings]
```ini
m_bShowBuildingSockets=False
m_bMitigateAllSandstormDamage=False
m_bEnableStabilizationSystem=True
m_bEnableDestabilizationSystem=False
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
m_LandclaimThresholdDistance=512.000000
m_LandclaimFoundationDistanceFromBorder=35.000000
m_BuildingBlueprintMaxExtensions=4
m_BaseBackupMaxExtensions=8
m_bBuildingRestrictionLimitsEnabled=True
m_bCanRemoveBuildablesWithNoOwner=True

; **STAKING UNIT EXTENSION TIMES** (MISSED IN ORIGINAL)
; Horizontal staking units - extension times in seconds
+m_StakingUnitExtensionDefaultTimes=60.000000      ; 1 minute
+m_StakingUnitExtensionDefaultTimes=120.000000     ; 2 minutes
+m_StakingUnitExtensionDefaultTimes=240.000000     ; 4 minutes
+m_StakingUnitExtensionDefaultTimes=480.000000     ; 8 minutes
+m_StakingUnitExtensionDefaultTimes=960.000000     ; 16 minutes
+m_StakingUnitExtensionDefaultTimes=1920.000000    ; 32 minutes
+m_StakingUnitExtensionDefaultTimes=3840.000000    ; 64 minutes (~1 hour)
+m_StakingUnitExtensionDefaultTimes=7680.000000    ; 128 minutes (~2 hours)
+m_StakingUnitExtensionDefaultTimes=15360.000000   ; 256 minutes (~4.3 hours)
+m_StakingUnitExtensionDefaultTimes=30720.000000   ; 512 minutes (~8.5 hours)

; Vertical staking units - same extension times
+m_StakingUnitVerticalExtensionDefaultTimes=60.000000
+m_StakingUnitVerticalExtensionDefaultTimes=120.000000
+m_StakingUnitVerticalExtensionDefaultTimes=240.000000
+m_StakingUnitVerticalExtensionDefaultTimes=480.000000
+m_StakingUnitVerticalExtensionDefaultTimes=960.000000
+m_StakingUnitVerticalExtensionDefaultTimes=1920.000000
+m_StakingUnitVerticalExtensionDefaultTimes=3840.000000
+m_StakingUnitVerticalExtensionDefaultTimes=7680.000000
+m_StakingUnitVerticalExtensionDefaultTimes=15360.000000
+m_StakingUnitVerticalExtensionDefaultTimes=30720.000000

m_StakingUnitType=(Name="StakingUnit_Placeable")
m_StakingUnitVerticalType=(Name="StakingUnitVertical_Placeable")

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
m_PentashieldDetectionDistanceCharacterAudio=5000.000000
m_PentashieldDetectionDistanceCharacter=500.000000
m_SmallRangeDoorDetectionDistance=250.000000
m_OpenDoorMinimumVelocity=600.000000
m_TimeToAutomaticallyCloseDoor=10
m_TimeToAutomaticallyCloseDoorRetry=1

; Refund/repair
m_DefaultRepairCostMultiplier=0.500000
m_DefaultBuildingSystemModifiers=(m_RefundPercentage=1.000000,m_PlacementCostMultiplier=1.000000)
m_PickupTotalDurabilityPercentageReduction=0.050000

; Server border building
m_bEnableBuildingNearServerBorders=False
m_bMinBuildableDistanceFromServerBorder=1000.000000
; DeepDesert custom: 10000.0

; Free placement limits
m_FreeTranslateMax=100.000000
m_FreeRotateMax=45.000000
m_FreeRotateSpeed=5.000000
m_FreeTranslateSpeed=100.000000
m_BuildRange=2000.000000
m_BuildingHeightLimitInM=980.000000

; Blueprint backup restrictions
m_BaseBackupToolTimeRestrictionInSeconds=604800    ; 7 days
m_BuildingBlueprintSnapToOriginBaseBackupMaxAllowedDistance=5000.0
```

### [/Script/DuneSandbox.DuneAISettings]
```ini
m_MaxReinforcementSize=150.000000
m_NavigableLocationSearchRadius=300.000000
m_ReinforcementHeightAboveGround=1000.000000
m_MinAttackDelayTime=0.200000
m_MaxAttackDelayTime=5.000000

; Threat decay
m_ThreatDecayPerSecond=0.100000
m_ThreatDecayCooldown=1.000000

; NPC LOD settings
m_EntityLod0Radius=3750.000000
m_EntityLod1Radius=10000.000000
m_EntityLod2Radius=15000.000000

; Respawn
m_PVPRespawn=(m_FallbackRespawnTimeMinutes=8.0)
m_DefaultRespawn=(m_FallbackRespawnTimeMinutes=8.0)

; NPC behavior
m_CorpseLifespanInSeconds=120.000000
m_GhostModeMaxTimeInSeconds=30.000000
m_WasRecentlyStaggeredMaxTimeInSeconds=5.000000
m_WasRecentlyDamagedByTargetMaxTimeInSeconds=1.000000
m_RandomDBNOChance=0.100000
```

### [/Script/DuneSandbox.SandStormConfig]
```ini
m_bAutoSpawnEnabled=True
m_bSandStormDebrisEnabled=True
m_SandStormDebrisSpeed=3000.000000
m_PlayerOverlapCheckIntervalInSeconds=1.000000
m_BuildingOverlapCheckIntervalInSeconds=5.000000
m_PlaceableOverlapCheckIntervalInSeconds=5.000000
m_VehicleOverlapCheckIntervalInSeconds=3.000000
m_DamageFramesPerOverlapInterval=15
m_NetCullDistanceInMeters=10000.000000
m_FadeDistanceInMeters=9000.000000

; Coriolis storm settings
m_bCoriolisAutoSpawnEnabled=True
m_CoriolisSpawnWarningsDurationInHours=6
m_CoriolisStage1DurationInSeconds=32400.000000    ; 9 hours
m_CoriolisStage2DurationInSeconds=3540.000000     ; 59 minutes
m_CoriolisStage3DurationSeconds=60.000000         ; 1 minute
m_CoriolisStage4DurationSeconds=60.000000         ; 1 minute
m_CoriolisStage5DurationSeconds=1740.000000       ; 29 minutes
m_CoriolisSandstormSpawnPreventionSeconds=600.000000
m_bCoriolisDoesDamage=False
m_bCoriolisTriggerShiftingSands=False
m_CoriolisLightDamage=5.000000
m_CoriolisHeavyDamage=5000.000000
m_TotemShortCircuitTimeAfterCoriolis=300.000000   ; 5 minutes
m_TotemShortCircuitTimeAfterSandstorm=300.000000

; Damage per tick (small/large storm)
m_SmallSandStormDamageConfig=(Player=5.0,Building=5.0,Placeable=5.0,Vehicle=5.0)
m_LargeSandStormDamageConfig=(Player=7.0,Building=7.0,Placeable=7.0,Vehicle=7.0)
```

### [/Script/DuneSandbox.DuneVehicleSettings]
```ini
m_LastDamageDealtTimeThreshold=1.000000
m_VehicleAccessTokenDuration=120.000000
m_OrnithopterInAirDistanceToGround=300.000000
Vehicle.CollisionDamageReductionFactor=0.010000
Vehicle.CollisionDamageReductionCooldownSpeed=1.000000

; Recovery cost multipliers per vehicle (in RecoveryPerVehicleClassCurrencyMultipliers)
; Sandbike=0.2, TreadWheel=0.2, Buggy=0.6, LightOrni=1.0, MediumOrni=1.2
; TransportOrni=2.0, SandCrawler=2.0

; Time for vehicle wreck to drop a loot blob (in m_TimeToDropWreckBlobPerVehicleMap)
; All vehicles: 7200.0 seconds (2 hours)
```

### [/Script/DuneSandbox.CraftingSettings]
```ini
m_MaximumNumberOfCharges=999
m_MaximumNumberOfRequestsPerRecipe=1000
m_CostCheatMaxRecipes=500
m_MaxArraySerializationSize=500
m_ListenResourcesResponseLimit=100
m_ListenResourcesRequestCooldownTime=5.000000
m_RepairCostWeight=0.500000
m_RecyclerOutputWeight=0.250000
```

### [/Script/DuneSandbox.HydrationSubsystem]
```ini
m_bHydrationEnabled=True
m_BiomeTierUpdateRateSeconds=2.5
```

### [/Script/DuneSandbox.BiomeSettings]
```ini
m_SandBuildupMultiplier=1.0
```

### [/Script/DuneSandbox.ShelterSettings]
```ini
m_ShelterTraceLength=10000.000000
m_ShelterTraceGridCellSize=200.000000
m_ShelterTraceFrameBudget=350
m_BuildingShelterThreshold=0.9
m_PlaceableShelterThreshold=0.65
m_bUseShelterGeneralTraceOnly=false
```

### [/Script/DuneSandbox.PingSystemSettings]
```ini
m_PingsPerPlayerLimit=5
m_PingMaximumDistance=2000.000000
m_PingPlacementHeightOffset=0.500000
m_QuickPingDoubleTapActionDelayInSeconds=0.300000
m_PingInWorldMarkerExpiryTime=5
m_PingMapMarkerExpiryTime=60
```

### [/DeteriorationSystem.ItemDeteriorationConstants]
```ini
UpdateRateInSeconds=1.0
```

### [/Script/DuneSandbox.FlourSandSubsystem]
```ini
m_FlourSandFieldsActivePercentage=1.0
```

### [/Script/DuneSandbox.DewHarvestSettings]
```ini
m_DewRefreshTime=12.000000
m_DewRefreshTimeNPE=300.0
```

### [/Script/DuneSandbox.GuildSettings]
```ini
m_GuildCreationCost=1000
m_MaxGuildsAllowed=3
m_MaxGuildMembersAllowed=32
m_MaxPendingGuildInvitesAllowed=10
```

### [/Script/DuneSandbox.LandsraadSettings]
```ini
bIsLandsraadEnabled=True
m_TaskGoalAmount=70000
m_ControlPointsPerCycle=2
m_LandsraadContractsPerVotingBlock=3
m_LandsraadContractsRepeatCooldownSeconds=57600     ; 16 hours
m_LandsraadContractsMaxActiveAmount=3
m_LandsraadContractsAbandonCooldownSeconds=3600     ; 1 hour
m_LandsraadContractsDailyBonusPerDay=5
m_LandsraadContractsDailyBonusMax=35
```

### [/Script/DuneSandbox.ResourceLocationSystem]
```ini
m_bIsEnabled=True
m_ResourcePointTrace=MoveUpwards
m_ResourceSpawnChance=1.0
```

### [/Script/DuneSandbox.InventorySystemSettings]
```ini
PlayerInventoryStartingSize=35
PlayerInventoryColumnCount=5
PlayerInventoryStartingVolumeCapacity=175.000000
P2pTradingInventoryStartingSize=10
DecayedMaxDurabilityThreshold=0.200000
LootContainerDistanceThreshold=500.000000
PerPlayerLootHiddemItemRefreshTime=5.000000
PerPlayerLootMinimumDespawnTimeAfterInteraction=30.000000
MaxLootDifficultyLevel=40
MaxLootQualityLevel=5
VendorBaselineDemand=0.050000
MaxSqrDistanceToVendor=250000.000000
MaxVendorCycleDuration=2419200                      ; ~28 days
MaxSlotlessItemBuyAmountPerBulk=40
```

### [/Script/DuneSandbox.LootSettings]
```ini
GlobalLootRightsBehaviour=PerPlayerChestAndNpcDrop
```

### [/Script/DuneSandbox.FactionSettings]
```ini
m_FactionTierLock=2
```

### [/Script/DuneSandbox.CharacterRecustomizerSubsystem]
```ini
m_CostAmount=5000                                    ; Solaris cost to recustomize
```

### [/Script/DuneSandbox.PartySettings]
```ini
m_SocialRange=1000000.000000
```

### [/Script/DuneSandbox.PlayerOnlineStateSettings]
```ini
m_DefaultReconnectGracePeriodSeconds=300
m_OvermapReturnGracePeriodSeconds=90
m_InstancedMapReconnectGracePeriodSeconds=300
```

### [/Script/DuneSandbox.DunePlayerCharacter]
```ini
s_RepeatedKillCooldown=300.0                        ; 5 minutes between same-player kills counting
```

### [/Script/DuneSandbox.DuneVehicle]
```ini
s_RecentlyDrivenTime=15.0                           ; Seconds vehicle considered "recently driven"
s_CombatRatingScalar=16
m_VehicleShelterThreshold=0.75
```

### [/Script/DuneSandbox.DuneCharacter]
```ini
m_ShelterThreshold=0.75
m_ExitShootingSubstateDelayDuration=1.0
```

### [/Script/DuneSandbox.DuneNpcCharacter]
```ini
s_CombatRatingScalar=16.0
```

### [/Script/DuneSandbox.DunePlayerController]
```ini
m_RespawnGUITimer=2.0
m_SandwormDeathRespawnGUITimer=5.0
```

### [/Script/DuneSandbox.PatrolShipSubSystem]
```ini
m_TimeOfDayToSpawn=18.000000                        ; 6 PM
m_TimeOfDayToDespawn=6.000000                       ; 6 AM
```

### [/Script/DuneSandbox.HazardsSettings]
```ini
m_VehicleQuicksandDamage=10000.000000
m_SandwormQuicksandSpeedModifier=0.250000
m_DeathDelayDuration=3.000000
m_CharacterMaxDepthEffectsDelayDuration=5.500000
m_VehicleMaxDepthEffectsDelayDuration=5.500000
```

### [/Script/DuneSandbox.AugmentSettings]
```ini
m_MinimumAugmentableItemQuality=0
m_JackpotRollPercentage=0.950000
m_PercentageStepInterval=0.001000
m_PercentageDecimalCaseThreshold=0.050000
m_MaxRangedWeaponAugments=3
m_MaxMeleeWeaponAugments=3
m_MaxArmorAugments=2
```

### [/Script/DuneSandbox.TechKnowledgeSettings]
```ini
m_bRevealItemOnDistributedToCharacter=False
+m_SuggestedCategoryTiers=0
+m_SuggestedCategoryTiers=22
+m_SuggestedCategoryTiers=100
+m_SuggestedCategoryTiers=300
+m_SuggestedCategoryTiers=700
+m_SuggestedCategoryTiers=1200
```

---

## PART 2: Network Settings (Engine)

### [/Script/Engine.GameNetworkManager]
```ini
ClientErrorUpdateRateLimit=0.35
ClientNetSendMoveDeltaTime=0.0333
ClientNetSendMoveDeltaTimeThrottled=0.0666
ClientNetSendMoveDeltaTimeStationary=0.0833
ClientnetSendMoveThrottleOverPlayerCount=30
MAXCLIENTUPDATEINTERVAL=0.35
MaxClientForcedUpdateDuration=0.60
ServerForcedUpdateHitchThreshold=0.10
ServerForcedUpdateHitchCooldown=0.30
MaxMoveDeltaTime=0.25
MAXPOSITIONERRORSQUARED=64
bMovementTimeDiscrepancyDetection=true
bMovementTimeDiscrepancyResolution=true
MovementTimeDiscrepancyMaxTimeMargin=1.5
MovementTimeDiscrepancyMinTimeMargin=-1.5
MovementTimeDiscrepancyResolutionRate=1.0
MovementTimeDiscrepancyDriftAllowance=0.03
bMovementTimeDiscrepancyForceCorrectionsDuringResolution=true
ClientNetCamUpdateDeltaTime=0.66
ClientNetCamUpdatePositionLimit=650
```

---

## PART 3: Map FPS Settings

### [/Script/DuneSandbox.MapFpsSettings]
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
| LostHarvest | 20 |
| ArtOfKanly | 20 |
| All Dungeons | 20 |

---

## PART 4: Map Capacity Settings

### [/Script/DuneSandbox.MatchmakerEventsSettings]
| Map | Max Players |
|-----|-------------|
| Survival_1 (sietch) | 40 |
| DeepDesert_1 | 100 |
| Overmap | 1000 |
| SH_Arrakeen | 100 |
| SH_HarkoVillage | 100 |
| Story_ArtOfKanly | 30 |
| Story instances | 1-4 |
| Dungeons | 4 |

---

## NOTES

1. **Staking Unit Times**: These control how long staking units can extend claims. Array values from 1 min to 8.5 hours.

2. **Array syntax**: Settings prefixed with `+` are UE4 array appends. To override, you typically need to clear the array first with `-` syntax or replace the entire array.

3. **Console Variables**: Most `m_` prefixed settings are class defaults, not CVARs. They may not all be runtime-modifiable.

4. **Testing**: Always test changes incrementally. Not all settings are guaranteed to work in the self-hosted server environment.
