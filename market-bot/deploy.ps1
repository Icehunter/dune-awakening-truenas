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
    [string]$VmIp = $(if ($env:DUNE_VM_IP) { $env:DUNE_VM_IP } else { "192.168.0.72" })
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir  = $PSScriptRoot
$SshKey     = Join-Path $ScriptDir "..\sshKey"
$VmUser     = "dune"
$RemoteDir  = "/opt/market-bot"
$DataDir    = Join-Path $ScriptDir "..\dune-admin"

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

# ── 2b. Drop existing bot orders ─────────────────────────────────────────────
Write-Host "==> Dropping existing bot orders..." -ForegroundColor Cyan
try {
    $dbInfo = & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl get pods -A --no-headers | grep 'db-dbdepl-sts-0'" 2>$null
    if ($dbInfo) {
        $parts  = ($dbInfo -split '\s+', 3)
        $dbNs   = $parts[0]; $dbPod = $parts[1]
        $sql    = "DELETE FROM dune.items WHERE id IN (SELECT item_id FROM dune.dune_exchange_orders WHERE owner_id = 158 AND is_npc_order = TRUE AND item_id IS NOT NULL); DELETE FROM dune.dune_exchange_orders WHERE owner_id = 158 AND is_npc_order = TRUE;"
        Invoke-Ssh "sudo kubectl exec -n $dbNs $dbPod -- psql -U dune -h localhost -p 15432 -d dune -c '$sql'"
        Write-Host "    done."
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
Invoke-Scp (Join-Path $DataDir "item-data.json")      "${VmUser}@${VmIp}:${RemoteDir}/data/item-data.json"
Invoke-Scp (Join-Path $DataDir "dune-item-names.json") "${VmUser}@${VmIp}:${RemoteDir}/data/dune-item-names.json"

# ── 5. Apply k8s manifest ─────────────────────────────────────────────────────
Write-Host "==> Applying k8s manifests..." -ForegroundColor Cyan
$manifest = Get-Content (Join-Path $ScriptDir "k8s\market-bot.yaml") -Raw
$manifest | & ssh -o StrictHostKeyChecking=no -i $SshKey "${VmUser}@${VmIp}" "sudo kubectl apply -f -"
if ($LASTEXITCODE -ne 0) { throw "kubectl apply failed" }

# ── 6. Rollout ────────────────────────────────────────────────────────────────
Write-Host "==> Restarting deployment..." -ForegroundColor Cyan
Invoke-Ssh "sudo kubectl rollout restart deployment/market-bot -n dune-market-bot"
Invoke-Ssh "sudo kubectl rollout status deployment/market-bot -n dune-market-bot --timeout=90s"

# ── 7. Logs ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "==> Logs:" -ForegroundColor Cyan
Invoke-Ssh "sudo kubectl logs -n dune-market-bot -l app=market-bot --tail=40"
