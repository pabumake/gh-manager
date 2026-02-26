param(
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"

$Repo = if ($env:GHM_REPO) { $env:GHM_REPO } else { "pabumake/gh-manager" }
$ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$BinName = "gh-manager.exe"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\gh-manager\bin"

function Get-Arch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    switch ($arch) {
        "x64" { return "amd64" }
        "arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

$dstExe = Join-Path $InstallDir $BinName

if ($Uninstall) {
    Write-Host "Uninstalling gh-manager..."
    if (Test-Path $dstExe) {
        try {
            & $dstExe theme apply default | Out-Null
            & $dstExe theme uninstall catppuccin-mocha | Out-Null
        } catch {
        }
    }
    if (Test-Path $dstExe) {
        Remove-Item -Path $dstExe -Force
    }

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath) {
        $cleaned = ($userPath -split ';' | Where-Object { $_ -and $_ -ne $InstallDir }) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $cleaned, "User")
    }
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")

    if ((Test-Path $InstallDir) -and -not (Get-ChildItem -Path $InstallDir -Force | Select-Object -First 1)) {
        Remove-Item -Path $InstallDir -Force
    }
    Write-Host "Uninstall complete."
    Write-Host "If PATH updates are not visible yet, open a new PowerShell session."
    exit 0
}

$arch = Get-Arch
$assetName = "gh-manager_windows_${arch}.zip"

Write-Host "Resolving latest release from $Repo..."
$release = Invoke-RestMethod -Uri $ApiUrl
$asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) {
    throw "Release asset not found: $assetName"
}

$tmpRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("gh-manager-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpRoot | Out-Null
try {
    $zipPath = Join-Path $tmpRoot $assetName
    $extractDir = Join-Path $tmpRoot "extract"
    New-Item -ItemType Directory -Path $extractDir | Out-Null

    Write-Host "Downloading $assetName..."
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath

    Write-Host "Extracting $assetName..."
    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

    $srcExe = Join-Path $extractDir ("gh-manager_windows_${arch}\gh-manager.exe")
    if (-not (Test-Path $srcExe)) {
        throw "Extracted binary not found: $srcExe"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $srcExe -Destination $dstExe -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $parts = @()
    if ($userPath) {
        $parts = $userPath -split ';'
    }
    if ($parts -notcontains $InstallDir) {
        $newPath = if ($userPath -and $userPath.Trim()) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    }

    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")

    Write-Host "Applying theme: catppuccin-mocha..."
    & $dstExe theme install catppuccin-mocha
    if ($LASTEXITCODE -ne 0) { throw "Theme install failed." }
    & $dstExe theme apply catppuccin-mocha
    if ($LASTEXITCODE -ne 0) { throw "Theme apply failed." }

    Write-Host "Install complete."
    Write-Host "Version: $(& $dstExe version)"
    Write-Host "$(& $dstExe theme current)"
    Write-Host "Binary: $dstExe"
    Write-Host "If PATH updates are not visible yet, open a new PowerShell session."
}
finally {
    if (Test-Path $tmpRoot) {
        Remove-Item -Path $tmpRoot -Recurse -Force
    }
}
