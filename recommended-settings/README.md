# Recommended Server Settings

Upload these files via **FileBrowser**: http://192.168.0.72:18888

Path: `UserSettings/` folder

## Files

| File | Purpose |
|------|---------|
| `UserEngine.ini` | Core server settings (resources, vehicles, weather) |
| `UserGame.ini` | Gameplay settings (PvP/PvE, building, loot) |

## Configuration Philosophy

These settings are tuned for a **private/friends server**:

### Resource Gathering: 2x
- Faster progression without being trivial
- Still requires effort to gather materials
- Vehicle mining also boosted

### Vehicle Durability: 0.5x
- Less repair grind
- Vehicles still need maintenance, just less often

### PvE-Focused
- PvP only in designated zones (not forced everywhere)
- Security zones remain safe
- Can enable PvP on specific sietches if desired

### Building: Expanded
- Larger land claims (8 segments vs default 6)
- More blueprint extensions
- More base backups

### Quality of Life
- Longer map marker duration
- Double loot from harder content
- Slower item decay

## Alternative Configurations

### Hardcore PvP Server
```ini
; In UserGame.ini
[/Script/DuneSandbox.PvpPveSettings]
m_bShouldForceEnablePvpOnAllPartitions=True

[/Script/DuneSandbox.SecurityZonesSubsystem]
m_bAreSecurityZonesEnabled=False
```

### Easy/Casual Mode
```ini
; In UserEngine.ini
[ConsoleVariables]
Dune.GlobalMiningOutputMultiplier=5.0
dw.VehicleDurabilityDamageMultiplier=0.1
sandworm.dune.Enabled=0
Sandstorm.Enabled=0

; In UserGame.ini
[/Script/DuneSandbox.ItemDeteriorationSettings]
m_ItemDeteriorationUpdateRate=0.1
```

### Vanilla-ish (Minimal Changes)
```ini
; In UserEngine.ini
[ConsoleVariables]
Bgd.ServerLoginPassword=""
; Everything else default
```

## Applying Changes

1. Upload files to `UserSettings/` via FileBrowser
2. Restart the battlegroup:
   ```bash
   ./battlegroup.sh restart
   ```

Changes take effect on server restart.
