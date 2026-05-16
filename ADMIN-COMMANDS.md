# Dune Awakening Admin/GM Commands Reference
## Self-Hosted Server Administration Guide

**⚠️ PRIVATE DOCUMENT** - Contains admin passwords and GM commands. Do not share publicly.

---

## Current Status

**Console Access**: The UE5 console is disabled in shipping builds at compile time. Config changes and keybinds don't help - the code path is removed.

**BattlEye**: Disabled on self-hosted servers (`BattlEye.Enabled=false` in DefaultEngine.ini)

**Potential Workarounds Being Tested**:
- Steam launch option: `-ExecCmds="AdminLogin sardaukar"`
- UE5 Console Unlocker tools (now possible with BattlEye disabled)
- Direct database manipulation

---

## Admin Configuration

### [AdminSetting.Global]
```ini
Password_Admin=sardaukar
+Allowed_Commands=suicide
+Allowed_Commands=AdminLogin
+Allowed_Commands=PrintAllowedCommands
+Allowed_Commands=ReportBugPlayerUI
+Allowed_Commands=ReportBugUI
+Allowed_Commands=RequestHelpUI
```

### Changing Admin Password

Add to your `UserGame.ini`:
```ini
[AdminSetting.Global]
Password_Admin=YourNewSecurePassword
```

---

## GM Commands (from DedicatedServerGame.ini)

These commands are whitelisted for admin use:

```ini
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

### Adding More GM Commands

Add to `UserGame.ini`:
```ini
[AdminSetting.Global]
+Allowed_GM_Commands=AwardXP
+Allowed_GM_Commands=SpawnNpc
+Allowed_GM_Commands=KillAllNpcs
+Allowed_GM_Commands=DestroyAllNpcs
+Allowed_GM_Commands=EncountersDestroyAndDisableAll
+Allowed_GM_Commands=SetAutoSandstormSpawnEnabled
+Allowed_GM_Commands=DestroyAllSandStorms
+Allowed_GM_Commands=ServerExec
+Allowed_GM_Commands=ResetProgression
+Allowed_GM_Commands=CleanPlayerInventory
+Allowed_GM_Commands=UpdateAllWaterFillables
+Allowed_GM_Commands=SkillsSetModuleLevel
+Allowed_GM_Commands=SkillsSetUnspentSkillPoints
+Allowed_GM_Commands=JourneyCompleteTaskByName
+Allowed_GM_Commands=CheatScript
```

---

## Cheat Modes Available

Once admin access is working, these cheat modes exist in the game:

| Mode | Effect |
|------|--------|
| `ECheatMode::God` | Full invulnerability |
| `ECheatMode::DemiGod` | Partial invulnerability |
| `ECheatMode::InfiniteAmmo` | Unlimited ammo |
| `ECheatMode::InfiniteAmmoWithReloading` | Unlimited ammo but must reload |
| `ECheatMode::InfiniteDurability` | Items don't break |
| `ECheatMode::InvisibleToNpc` | NPCs ignore you |
| `ECheatMode::DisableDehydration` | No water consumption |
| `ECheatMode::DisableColdSurvival` | No cold damage |
| `ECheatMode::DisableSpiceAddiction` | No spice addiction |
| `ECheatMode::DisableStamina` | Infinite stamina |
| `ECheatMode::DisablePower` | No power consumption |
| `ECheatMode::NoAbilityCost` | Abilities are free |

---

## Pre-defined Cheat Scripts

Built-in command sequences:

### LeaveMeAlone
```
CheatScript LeaveMeAlone
```
**Effect**: Destroys all NPCs, disables sandstorms, disables sandworms

**Internal commands**:
```
EncountersDestroyAndDisableAll
DestroyAllNpcs
SetAutoSandstormSpawnEnabled 0
DestroyAllSandStorms
ServerExec sandworm.dune.Enabled 0
```

### PlaytestSetup
```
CheatScript PlaytestSetup
```
**Effect**: Full T4 gear loadout, 10000 XP in each category, all skills unlocked

### AwardPlayerXP
```
CheatScript AwardPlayerXP
```
**Effect**: Awards 10000 Combat, Exploration, and Science XP

### UnlockAllSkills
```
CheatScript UnlockAllSkills
```
**Effect**: Sets all skill modules to level 1

### UnlockAllAbilities
```
CheatScript UnlockAllAbilities
```
**Effect**: Unlocks all active abilities (Blindspot, Hypersprint, Voice Compel, etc.)

---

## Command Examples

```bash
# Give items (use item TemplateId)
AddItemToInventory Ammo 1000
AddItemToInventory HeavyAmmo 500
AddItemToInventory HealthPack_Channeled_3 20
AddItemToInventory Solaris 10000

# Teleport
TeleportToPlayer <PlayerName>
TeleportToExact <X> <Y> <Z>
PrintPos                        # Shows your coordinates

# Flight modes
Fly                             # Enable flight
Ghost                           # Noclip mode  
Walk                            # Return to normal

# World manipulation
DestroyAllNpcs
DestroyAllSandStorms
SetAutoSandstormSpawnEnabled 0

# XP and progression
AwardXP Combat 50000
AwardXP Exploration 50000
AwardXP Science 50000
SkillsSetUnspentSkillPoints 100

# Server variables
ServerExec sandworm.dune.Enabled 0
ServerExec t.maxfps 30
```

---

## How to Access (When Working)

### Method 1: Console (if enabled)
1. Press `~` or `Insert` to open console
2. Type: `AdminLogin sardaukar`
3. Type: `PrintAllowedCommands` to verify

### Method 2: Steam Launch Options
```
-ExecCmds="AdminLogin sardaukar"
```

### Method 3: AdminPanel Widget
The game has an AdminPanel UI at:
```
/Game/Dune/GUI/Widgets/Menus/Gameplay/AdminPanel/W_AdminPanel.W_AdminPanel_C
```
Accessed via `ToggleAdminPanel` command (needs console first)

---

## Server-Side Config Files

| File | Purpose |
|------|---------|
| `DefaultEngine.ini` | `BattlEye.Enabled=false`, `bDeveloperMode=False` |
| `DefaultGame.ini` | Admin password, cheat scripts |
| `DedicatedServerGame.ini` | Allowed_GM_Commands whitelist |
| `DefaultInput.ini` | `ConsoleKeys=Tilde`, `ConsoleKeys=Insert` |

---

## Alternative: Database Direct Access

Since you have full kubectl/PostgreSQL access, you can:
- Add items directly to player inventories
- Modify player stats/XP
- Unlock skills/progression

```bash
./battlegroup.sh shell-pod
# Select database pod
psql -U dune -d dune
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Console won't open | Disabled in shipping build - need UE5 unlocker |
| Commands denied | Add to `+Allowed_GM_Commands` in UserGame.ini |
| Password not changing | Check UserGame.ini path, restart battlegroup |
| BattlEye blocking tools | It's disabled on self-hosted - shouldn't block |
| UUU crashes game | UE5 compatibility issue - try database method |

---

## Notes

- Default admin password is `sardaukar` - **CHANGE THIS**
- Commands are case-sensitive
- Some commands require specific player targeting
- The AdminPanel widget exists but UI access is blocked
- Database manipulation is the most reliable fallback
