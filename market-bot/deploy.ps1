#Requires -Version 5.1
<#
.SYNOPSIS
    Build and deploy the market-bot to the TrueNAS VM.

.DESCRIPTION
    Cross-compiles the bot for Linux/amd64, uploads the binary and item data
    via SCP, applies the k8s manifest, and rolls out the Deployment.

    On Windows the server typically runs in a Hyper-V VM. Find its IP with:
        Get-VM | Select-Object Name, @{n='IP';e={($_ | Get-VMNetworkAdapter).IPAddresses[0]}}

.PARAMETER VmIp
    IP address of the server VM (Hyper-V on Windows, TrueNAS on macOS/Linux).
    Defaults to the DUNE_VM_IP environment variable, or 192.168.0.72 if unset.

.EXAMPLE
    .\deploy.ps1 -VmIp 172.28.144.1
    $env:DUNE_VM_IP = "172.28.144.1"; .\deploy.ps1
#>
param(
    [string]$VmIp = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir    = $PSScriptRoot
$SshKey       = Join-Path $ScriptDir "..\sshKey"
$VmUser       = "dune"
$RemoteDir    = "/opt/market-bot"
$DataDir      = Join-Path $ScriptDir "..\dune-admin"
$DeployConfig = Join-Path $ScriptDir ".deploy-config"

# ── Resolve VM IP (param → env → cache → Hyper-V → prompt) ───────────────────
if (-not $VmIp) { $VmIp = $env:DUNE_VM_IP }
if (-not $VmIp -and (Test-Path $DeployConfig)) {
    $cached = Get-Content $DeployConfig | Where-Object { $_ -match '^DUNE_VM_IP=' } | Select-Object -First 1
    if ($cached) { $VmIp = $cached.Split('=', 2)[1].Trim() }
}
if (-not $VmIp) {
    try {
        $hvIp = Get-VM | Get-VMNetworkAdapter | Where-Object { $_.IPAddresses } |
                Select-Object -ExpandProperty IPAddresses |
                Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } |
                Select-Object -First 1
        if ($hvIp) {
            $VmIp = $hvIp
            Write-Host "    Hyper-V detected: $VmIp"
        }
    } catch {}
}
if (-not $VmIp) {
    $VmIp = Read-Host "Enter VM IP address"
    if (-not $VmIp) { throw "VM IP is required" }
}
# Cache whatever we resolved so future runs skip the detection.
if (-not (Test-Path $DeployConfig) -or -not (Select-String -Path $DeployConfig -Pattern '^DUNE_VM_IP=' -Quiet)) {
    Add-Content $DeployConfig "DUNE_VM_IP=${VmIp}"
    Write-Host "    cached VM IP: $VmIp"
}
Write-Host "==> Deploying to $VmIp..." -ForegroundColor Cyan

function Invoke-Ssh {
    param([string[]]$Command)
    & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" @Command
    if ($LASTEXITCODE -ne 0) { throw "SSH command failed (exit $LASTEXITCODE): $Command" }
}

function Invoke-Scp {
    param([string]$Source, [string]$Destination)
    & scp -o StrictHostKeyChecking=no -i $SshKey $Source $Destination
    if ($LASTEXITCODE -ne 0) { throw "SCP failed (exit $LASTEXITCODE)" }
}

function Get-ManifestValue {
    param([string]$Key)
    $line = Get-Content (Join-Path $ScriptDir "k8s\market-bot.yaml") |
            Where-Object { $_ -match "^\s*$([regex]::Escape($Key)):\s*" } |
            Select-Object -First 1
    if (-not $line) { return "" }
    return (($line -replace '^\s*[^:]+:\s*', '') -replace '^"|"$', '')
}

function ConvertTo-YamlString {
    param([string]$Value)
    return '"' + (($Value -replace '\\', '\\') -replace '"', '\"') + '"'
}

function Set-ManifestValue {
    param([string]$Manifest, [string]$Key, [string]$Value)
    $yamlValue = ConvertTo-YamlString $Value
    $pattern = "(?m)^(\s*$([regex]::Escape($Key)):\s*).*$"
    if ($Manifest -notmatch $pattern) { throw "missing manifest key: $Key" }
    return $Manifest -replace $pattern, "`${1}${yamlValue}"
}

# ── 1. Cross-compile ──────────────────────────────────────────────────────────
Write-Host "==> Cross-compiling for Linux/amd64..." -ForegroundColor Cyan
Push-Location $ScriptDir
try {
    $env:GOOS       = "linux"
    $env:GOARCH     = "amd64"
    $env:CGO_ENABLED = "0"
    & go build -trimpath -ldflags="-s -w" -o market-bot-linux .
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    $size = (Get-Item "market-bot-linux").Length / 1MB
    Write-Host "    built: $([math]::Round($size, 1)) MB"
} finally {
    Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}

# ── 2. Prepare remote directories ─────────────────────────────────────────────
Write-Host "==> Preparing remote directories..." -ForegroundColor Cyan
Invoke-Ssh "sudo mkdir -p ${RemoteDir}/{data,cache,bin} && sudo chown -R ${VmUser}:${VmUser} ${RemoteDir}"

# ── 2a. Detect or load cached DB host ─────────────────────────────────────────
$DetectedDbHost = ""

Write-Host "==> Detecting DB host from cluster..." -ForegroundColor Cyan
if (Test-Path $DeployConfig) {
    $cached = Get-Content $DeployConfig | Where-Object { $_ -match '^DUNE_DB_HOST=' } | Select-Object -First 1
    if ($cached) {
        $DetectedDbHost = $cached.Split('=', 2)[1].Trim()
        Write-Host "    using cached: $DetectedDbHost"
        Write-Host "    (delete $DeployConfig to re-detect)"
    }
}
if (-not $DetectedDbHost) {
    try {
        $svcLine = & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl get svc -A --no-headers 2>/dev/null | grep 'db-dbdepl-svc'" 2>$null
        if ($svcLine) {
            $parts = ($svcLine -split '\s+', 3)
            $svcNs = $parts[0]; $svcName = $parts[1]
            $DetectedDbHost = "${svcName}.${svcNs}.svc.cluster.local"
            Add-Content $DeployConfig "DUNE_DB_HOST=${DetectedDbHost}"
            Write-Host "    detected and cached: $DetectedDbHost"
        } else {
            Write-Host "    warn: DB service not found, using value from k8s/market-bot.yaml"
        }
    } catch {
        Write-Host "    warn: detection failed: $_" -ForegroundColor Yellow
    }
}

# Write detected host back to k8s/market-bot.yaml so the file stays in sync.
if ($DetectedDbHost) {
    $yamlPath = Join-Path $ScriptDir "k8s\market-bot.yaml"
    $yaml = Get-Content $yamlPath -Raw
    $yaml = $yaml -replace '(?m)^(\s*DB_HOST:\s*).*$', "`${1}${DetectedDbHost}"
    $yaml | Set-Content $yamlPath -NoNewline
}

# ── 2b. Detect DB credentials ────────────────────────────────────────────────
$DetectedDbUser = ""
$DetectedDbPass = ""
$DetectedDbName = ""
$DetectedDbPort = ""

Write-Host "==> Detecting DB credentials from cluster..." -ForegroundColor Cyan
try {
    $dbEnvInfo = & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl get pods -A --no-headers 2>/dev/null | grep 'db-dbdepl-sts-0'" 2>$null
    if ($dbEnvInfo) {
        $parts = ($dbEnvInfo -split '\s+', 3)
        $envNs = $parts[0]; $envPod = $parts[1]
        $dbEnv = & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl exec -n $envNs $envPod -- printenv" 2>$null
        foreach ($line in $dbEnv) {
            if ($line -match '^POSTGRES_USER=(.*)$') { $DetectedDbUser = $Matches[1] }
            if ($line -match '^POSTGRES_PASSWORD=(.*)$') { $DetectedDbPass = $Matches[1] }
            if ($line -match '^POSTGRES_DB=(.*)$') { $DetectedDbName = $Matches[1] }
            if ($line -match '^PGPORT=(.*)$') { $DetectedDbPort = $Matches[1] }
        }
    } else {
        Write-Host "    warn: DB pod not found, using DB credentials from k8s/market-bot.yaml"
    }
} catch {
    Write-Host "    warn: credential detection failed: $_" -ForegroundColor Yellow
}

$DbHost = if ($env:DUNE_DB_HOST) { $env:DUNE_DB_HOST } elseif ($DetectedDbHost) { $DetectedDbHost } else { Get-ManifestValue "DB_HOST" }
$DbPort = if ($env:DUNE_DB_PORT) { $env:DUNE_DB_PORT } elseif ($DetectedDbPort) { $DetectedDbPort } else { Get-ManifestValue "DB_PORT" }
$DbUser = if ($env:DUNE_DB_USER) { $env:DUNE_DB_USER } elseif ($DetectedDbUser) { $DetectedDbUser } else { Get-ManifestValue "DB_USER" }
$DbPass = if ($env:DUNE_DB_PASS) { $env:DUNE_DB_PASS } elseif ($DetectedDbPass) { $DetectedDbPass } else { Get-ManifestValue "DB_PASS" }
$DbName = if ($env:DUNE_DB_NAME) { $env:DUNE_DB_NAME } else { Get-ManifestValue "DB_NAME" }

if (-not $DbPort) { $DbPort = "15432" }
if (-not $DbName) {
    if ($DetectedDbName) { $DbName = $DetectedDbName } else { $DbName = "dune" }
}
if (-not $DbHost -or -not $DbUser -or -not $DbPass) {
    throw "could not detect complete DB credentials. Set DUNE_DB_HOST, DUNE_DB_USER, DUNE_DB_PASS, and optionally DUNE_DB_PORT/DUNE_DB_NAME."
}

Write-Host "    user: $DbUser"
Write-Host "    database: $DbName"
Write-Host "    port: $DbPort"
Write-Host "    password: detected"

# ── 2c. Drop existing bot orders ─────────────────────────────────────────────
Write-Host "==> Dropping existing bot orders..." -ForegroundColor Cyan
try {
    $dbInfo = & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl get pods -A --no-headers | grep 'db-dbdepl-sts-0'" 2>$null
    if ($dbInfo) {
        $parts  = ($dbInfo -split '\s+', 3)
        $dbNs   = $parts[0]; $dbPod = $parts[1]
        $sql = @"
WITH bot AS (SELECT id FROM dune.actors WHERE class = 'Revy' LIMIT 1),
del_orders AS (
  DELETE FROM dune.dune_exchange_orders
  WHERE owner_id = (SELECT id FROM bot) AND is_npc_order = TRUE
  RETURNING item_id
),
del_items AS (
  DELETE FROM dune.items
  WHERE id IN (SELECT item_id FROM del_orders WHERE item_id IS NOT NULL)
  RETURNING id
)
SELECT (SELECT COUNT(*) FROM del_orders) AS orders_deleted,
       (SELECT COUNT(*) FROM del_items)  AS items_deleted;
"@
        $sql | & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl exec -n $dbNs $dbPod -i -- psql -U dune -h localhost -p 15432 -d dune"
    } else {
        Write-Host "    warn: DB pod not found, skipping order cleanup."
    }
} catch {
    Write-Host "    warn: order cleanup failed: $_" -ForegroundColor Yellow
}

# ── 3. Upload binary ──────────────────────────────────────────────────────────
Write-Host "==> Uploading binary..." -ForegroundColor Cyan
Invoke-Scp (Join-Path $ScriptDir "market-bot-linux") "${VmUser}@${VmIp}:/tmp/market-bot-new"
Invoke-Ssh "sudo mv /tmp/market-bot-new ${RemoteDir}/bin/market-bot && sudo chmod +x ${RemoteDir}/bin/market-bot"

# ── 4. Upload item data ───────────────────────────────────────────────────────
Write-Host "==> Uploading item data..." -ForegroundColor Cyan
Invoke-Scp (Join-Path $DataDir "item-data.json") "${VmUser}@${VmIp}:${RemoteDir}/data/item-data.json"

# ── 5. Apply k8s manifest ─────────────────────────────────────────────────────
Write-Host "==> Applying k8s manifests..." -ForegroundColor Cyan
$manifest = Get-Content (Join-Path $ScriptDir "k8s\market-bot.yaml") -Raw
$manifest = Set-ManifestValue $manifest "DB_HOST" $DbHost
$manifest = Set-ManifestValue $manifest "DB_PORT" $DbPort
$manifest = Set-ManifestValue $manifest "DB_USER" $DbUser
$manifest = Set-ManifestValue $manifest "DB_PASS" $DbPass
$manifest = Set-ManifestValue $manifest "DB_NAME" $DbName
$manifest | & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl apply -f -"
if ($LASTEXITCODE -ne 0) { throw "kubectl apply failed" }

# ── 6. Rollout ────────────────────────────────────────────────────────────────
Write-Host "==> Restarting deployment..." -ForegroundColor Cyan
Invoke-Ssh "sudo kubectl rollout restart deployment/market-bot -n dune-market-bot"
& ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl rollout status deployment/market-bot -n dune-market-bot --timeout=90s" 2>&1 | Out-Host

# ── 7. Logs ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "==> Logs:" -ForegroundColor Cyan
Invoke-Ssh "sudo kubectl logs -n dune-market-bot -l app=market-bot --tail=40"
