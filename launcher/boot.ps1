#requires -Version 5.1
<#
.SYNOPSIS
    UML launcher for Windows — boots the User-Mode Linux VM through WSL2.

.DESCRIPTION
    UML kernels are Linux ELF binaries, so on Windows everything runs inside
    WSL2; this script is the Windows-native entry point. It mirrors boot:
    same argument order, same config.yaml next to the binaries.

        .\boot.ps1           # boots with config.yaml defaults (2G RAM)
        .\boot.ps1 4G 2      # custom memory and vCPUs

    Port forwards from config.yaml (ports: 2222:22, ...) bind inside WSL2;
    with localhostForwarding (on by default) they are reachable from Windows
    as localhost:2222.

.PARAMETER Memory
    Guest RAM, e.g. 512M, 2G, 8G. Optional — config.yaml / 2G default.

.PARAMETER Cpus
    Guest vCPUs. Optional — config.yaml / 1 default.

.PARAMETER Distro
    WSL distribution to run in. Default: the system default distro.
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)][string]$Memory,
    [Parameter(Position = 1)][int]$Cpus = 0,
    [string]$Distro = ""
)

$ErrorActionPreference = "Stop"
$Base = Split-Path -Parent $MyInvocation.MyCommand.Path

function Fail($msg) {
    Write-Host "[ERROR] $msg" -ForegroundColor Red
    exit 1
}

# --- WSL2 must exist ------------------------------------------------------
$wsl = Get-Command wsl.exe -ErrorAction SilentlyContinue
if (-not $wsl) {
    Fail "wsl.exe not found. Install WSL2 first: wsl --install (then reboot). See docs/windows.md."
}
$wslArgs = @()
if ($Distro -ne "") { $wslArgs += @("-d", $Distro) }

# --- required files (checked from the Windows side first) -----------------
foreach ($f in @("linux", "base.img", "vdeplug-netstack", "boot")) {
    if (-not (Test-Path (Join-Path $Base $f))) {
        Fail "Missing: $f — put this script next to the UML kernel, base image, network engine and boot script."
    }
}

# --- translate the script directory to a WSL path -------------------------
# Files may live on a Windows drive (C:\uml -> /mnt/c/uml) or already inside
# the WSL filesystem (\\wsl$\...); wslpath handles both.
$wslBase = (& wsl.exe @wslArgs -e wslpath -a -u "$Base").Trim()
if ($LASTEXITCODE -ne 0 -or $wslBase -eq "") {
    Fail "cannot translate '$Base' to a WSL path. Is a WSL2 distro installed? Run: wsl --list --verbose"
}

Write-Host "========================================"
Write-Host "  UML Virtual Machine (Windows / WSL2)"
Write-Host "----------------------------------------"
Write-Host "  Base dir : $Base"
Write-Host "  WSL path : $wslBase"
if ($Distro -ne "") { Write-Host "  Distro   : $Distro" }
Write-Host "----------------------------------------"

# --- exec bits don't survive a Windows copy; restore them -----------------
& wsl.exe @wslArgs -e sh -c "cd '$wslBase' && chmod +x linux vdeplug-netstack boot vde_plug 2>/dev/null || true"
if ($LASTEXITCODE -ne 0) { Fail "failed to prepare binaries inside WSL" }

# --- hand over to the Linux launcher --------------------------------------
$bootArgs = @()
if ($Memory -ne "") { $bootArgs += $Memory }
if ($Cpus -gt 0) {
    if ($bootArgs.Count -eq 0) { $bootArgs += "" }  # keep argv positions: boot <mem> <ncpus>
    $bootArgs += "$Cpus"
}
$quoted = ($bootArgs | ForEach-Object { "'$_'" }) -join " "
& wsl.exe @wslArgs --cd "$wslBase" -e sh -c "./boot $quoted"
exit $LASTEXITCODE
