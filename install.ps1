#
# oh-my-logs (oml) — Windows Installer (PowerShell)
#
[CmdletBinding()]
param(
    [switch]$Uninstall,
    [switch]$Nightly,
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\oh-my-logs",
    [string]$Version = "latest"
)

if ($env:OML_NIGHTLY -eq "1" -or $env:OML_NIGHTLY -eq "true" -or $Nightly) {
    $Nightly = $true
    $Version = "nightly"
} elseif ($env:OML_VERSION -and $Version -eq "latest") {
    $Version = $env:OML_VERSION
}

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

# 1. Determine architecture
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

# 2. Locate local binary, build from source, or download prebuilt release
$Repo = "brenonsantos/oh-my-logs"
$ScriptDir = if ($MyInvocation.MyCommand.Path) { Split-Path -Parent $MyInvocation.MyCommand.Path } else { "" }
$SourceBinary = if ($ScriptDir) { Join-Path $ScriptDir "oml.exe" } else { "" }
$ExtractedExamplesDir = ""

if ($SourceBinary -and (Test-Path $SourceBinary)) {
    Write-Step "Using local binary at $SourceBinary..."
} elseif ($ScriptDir -and (Get-Command "go" -ErrorAction SilentlyContinue) -and (Test-Path (Join-Path $ScriptDir "cmd\oml"))) {
    Write-Step "Building oml.exe from source..."
    Push-Location $ScriptDir
    try {
        go build -ldflags="-s -w" -o oml.exe ./cmd/oml
        $SourceBinary = Join-Path $ScriptDir "oml.exe"
    } finally {
        Pop-Location
    }
} else {
    # Download prebuilt binary from GitHub Releases
    Write-Step "Fetching prebuilt release for Windows ($Arch) from GitHub..."
    $TempDir = Join-Path $env:TEMP "oml-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

    try {
        $Headers = @{ "User-Agent" = "oh-my-logs-installer" }
        $ReleaseUrl = if ($Version -eq "latest") {
            "https://api.github.com/repos/$Repo/releases/latest"
        } else {
            "https://api.github.com/repos/$Repo/releases/tags/$Version"
        }

        $Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers $Headers
        $Pattern = "*windows_${Arch}.zip"
        $Asset = $Release.assets | Where-Object { $_.name -like $Pattern } | Select-Object -First 1

        if (-not $Asset) {
            Write-Error "Could not find release asset matching $Pattern in release $($Release.tag_name)."
            exit 1
        }

        $DownloadUrl = $Asset.browser_download_url
        $ZipPath = Join-Path $TempDir "oml.zip"

        Write-Step "Downloading $($Asset.name)..."
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing

        Write-Step "Extracting archive..."
        Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

        # Locate oml.exe in extracted folder
        $FoundBinary = Get-ChildItem -Path $TempDir -Recurse -Filter "oml.exe" | Select-Object -First 1
        if (-not $FoundBinary) {
            Write-Error "Extracted archive did not contain oml.exe."
            exit 1
        }
        $SourceBinary = $FoundBinary.FullName

        # Locate profiles in extracted folder
        $FoundProfiles = Get-ChildItem -Path $TempDir -Recurse -Directory -Filter "profiles" | Select-Object -First 1
        if (-not $FoundProfiles) {
            $FoundProfiles = Get-ChildItem -Path $TempDir -Recurse -Directory -Filter "examples" | Select-Object -First 1
        }
        if ($FoundProfiles) {
            $ExtractedProfilesDir = $FoundProfiles.FullName
        }
    } catch {
        Write-Error "Failed to download release: $_"
        exit 1
    }
}

# 3. Create destination directory & copy binary
Write-Step "Installing oml.exe to $InstallDir..."
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -Path $SourceBinary -Destination $BinaryPath -Force
Write-Success "Installed binary at $BinaryPath"

# 4. Add to User PATH if not already present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathParts = if ($UserPath) { $UserPath -split ";" } else { @() }
$AlreadyInPath = $PathParts | Where-Object { $_.TrimEnd("\") -ieq $InstallDir.TrimEnd("\") }

if (-not $AlreadyInPath) {
    Write-Step "Adding $InstallDir to User PATH..."
    $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Success "Added to User PATH"
} else {
    Write-Success "$InstallDir is already in User PATH"
}

# 5. Install default curated profiles (Zephyr, Logcat, Raw)
if (-not (Test-Path $ProfilesDir)) {
    New-Item -ItemType Directory -Path $ProfilesDir -Force | Out-Null
}

$ProfilesSource = if ($ExtractedProfilesDir) { $ExtractedProfilesDir } elseif ($ScriptDir -and (Test-Path (Join-Path $ScriptDir "examples\profiles"))) { Join-Path $ScriptDir "examples\profiles" } else { "" }
if ($ProfilesSource -and (Test-Path $ProfilesSource)) {
    $Copied = 0
    Get-ChildItem -Path $ProfilesSource -Filter "*.yaml" | ForEach-Object {
        $DestFile = Join-Path $ProfilesDir $_.Name
        if (-not (Test-Path $DestFile)) {
            Copy-Item -Path $_.FullName -Destination $DestFile
            $Copied++
        }
    }
    if ($Copied -gt 0) {
        Write-Success "Installed $Copied default profile(s) to $ProfilesDir"
    }
}

Write-Host "`noh-my-logs is ready to use!" -ForegroundColor Green
Write-Host "`nOpen a new PowerShell or Command Prompt window and try:" -ForegroundColor White
Write-Host "    oml                 # Open TUI serial monitor" -ForegroundColor Cyan
Write-Host "    oml --list-profiles # View available log profiles" -ForegroundColor Cyan
Write-Host "    oml --help          # See all options`n" -ForegroundColor Cyan
