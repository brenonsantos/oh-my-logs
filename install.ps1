#
# oh-my-logs (oml) — Windows Installer (PowerShell)
#
[CmdletBinding()]
param(
    [switch]$Uninstall,
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\oh-my-logs"
)

$ErrorActionPreference = "Stop"

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-WarningMsg {
    param([string]$Message)
    Write-Host "! $Message" -ForegroundColor Yellow
}

$ProfilesDir = "$env:APPDATA\oh-my-logs\profiles"
$BinaryPath = Join-Path $InstallDir "oml.exe"

if ($Uninstall) {
    Write-Step "Uninstalling oh-my-logs (oml)..."
    if (Test-Path $BinaryPath) {
        Remove-Item -Path $BinaryPath -Force
        Write-Success "Removed $BinaryPath"
    }

    # Remove from User PATH
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -like "*$InstallDir*") {
        $CleanedPath = ($UserPath -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ne $InstallDir.TrimEnd('\') }) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $CleanedPath, "User")
        Write-Success "Removed $InstallDir from User PATH"
    }

    Write-Host "`nNote: Profiles in $ProfilesDir were preserved." -ForegroundColor Gray
    exit 0
}

Write-Host "oh-my-logs (oml) Installer for Windows`n" -ForegroundColor White

# 1. Locate or build oml.exe
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceBinary = Join-Path $ScriptDir "oml.exe"

if (-not (Test-Path $SourceBinary)) {
    if (Get-Command "go" -ErrorAction SilentlyContinue) {
        Write-Step "Building oml.exe from source..."
        Push-Location $ScriptDir
        try {
            go build -o oml.exe ./cmd/oml
        } finally {
            Pop-Location
        }
    } else {
        Write-Error "Neither prebuilt 'oml.exe' nor 'go' compiler was found. Please install Go (https://go.dev) or build oml.exe first."
        exit 1
    }
}

# 2. Create destination directory & copy binary
Write-Step "Installing oml.exe to $InstallDir..."
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Copy-Item -Path $SourceBinary -Destination $BinaryPath -Force
Write-Success "Installed binary at $BinaryPath"

# 3. Add to User PATH if needed
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$NormalizedTarget = $InstallDir.TrimEnd('\')
$Paths = $UserPath -split ';' | ForEach-Object { $_.TrimEnd('\') }

if ($Paths -notcontains $NormalizedTarget) {
    Write-Step "Adding $InstallDir to User PATH..."
    $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Success "Added to User PATH"
} else {
    Write-Success "$InstallDir is already in User PATH"
}

# 4. Copy default profiles
if (-not (Test-Path $ProfilesDir)) {
    New-Item -ItemType Directory -Path $ProfilesDir -Force | Out-Null
}

$ExamplesDir = Join-Path $ScriptDir "profiles\examples"
if (Test-Path $ExamplesDir) {
    $Copied = 0
    Get-ChildItem -Path $ExamplesDir -Filter "*.yaml" | ForEach-Object {
        $DestFile = Join-Path $ProfilesDir $_.Name
        if (-not (Test-Path $DestFile)) {
            Copy-Item -Path $_.FullName -Destination $DestFile
            $Copied++
        }
    }
    if ($Copied -gt 0) {
        Write-Success "Copied $Copied default profile(s) to $ProfilesDir"
    }
}

Write-Host "`noh-my-logs is ready to use!" -ForegroundColor Green
Write-Host "`nOpen a new PowerShell or Command Prompt window and try:" -ForegroundColor White
Write-Host "    oml                 # Open TUI serial monitor" -ForegroundColor Cyan
Write-Host "    oml --list-profiles # View available log profiles" -ForegroundColor Cyan
Write-Host "    oml --help          # See all options`n" -ForegroundColor Cyan
