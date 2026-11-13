param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Tag,
    [Parameter(Mandatory = $true)][string]$AssetsDir,
    [Parameter(Mandatory = $false)][string]$Channel = "release",
    [Parameter(Mandatory = $false)][string]$Repository = "SnishaperTeam/SniShaper"
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem

$installerDir = Join-Path $RepoRoot 'installer'
$updaterRoot = Join-Path $RepoRoot 'build\bin\updater\Windows'
if (-not (Test-Path $installerDir)) {
    Write-Host "::error::installer directory not found: $installerDir"
    throw "installer directory not found"
}
if (-not (Test-Path $AssetsDir)) {
    New-Item -ItemType Directory -Path $AssetsDir -Force | Out-Null
}

$manifestChannel = switch ($Channel) {
    'alpha' { 'alpha' }
    'beta' { 'beta' }
    'rc' { 'rc' }
    default { 'stable' }
}
$utf8 = [System.Text.UTF8Encoding]::new($false)
$count = 0

foreach ($setup in Get-ChildItem -Path $installerDir -File -Filter '*Setup.exe') {
    if ($setup.Name -notmatch '-(x64|x86|arm64)Setup\.exe$') { continue }
    $arch = $Matches[1]
    $updater = Join-Path $updaterRoot "$arch\SniShaperUpdater.exe"
    if (-not (Test-Path $updater)) {
        Write-Host "::error::portable updater missing for ${arch}: $updater"
        throw "portable updater missing for $arch"
    }

    $zipName = "snishaper-$Version-windows-$arch-update.zip"
    $zipPath = Join-Path $AssetsDir $zipName
    if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
    $zip = [System.IO.Compression.ZipFile]::Open($zipPath, [System.IO.Compression.ZipArchiveMode]::Create)
    try {
        [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $setup.FullName, $setup.Name)
        [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $updater, 'SniShaperUpdater.exe')
        $config = Join-Path $updaterRoot "$arch\updater.json"
        if (Test-Path $config) {
            [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $config, 'updater.json')
        }
    } finally {
        $zip.Dispose()
    }

    $zipHash = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $setupHash = (Get-FileHash -LiteralPath $setup.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    $updaterHash = (Get-FileHash -LiteralPath $updater -Algorithm SHA256).Hash.ToLowerInvariant()

    $manifest = [ordered]@{
        version = $Version
        channel = $manifestChannel
        notes = "SniShaper $Version"
        package = [ordered]@{
            name = $zipName
            url = "https://github.com/$Repository/releases/download/$Tag/$zipName"
            sha256 = $zipHash
            size = (Get-Item -LiteralPath $zipPath).Length
        }
        installer = [ordered]@{
            name = $setup.Name
            sha256 = $setupHash
            size = $setup.Length
        }
        updater = [ordered]@{
            name = 'SniShaperUpdater.exe'
            sha256 = $updaterHash
        }
        published_at = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    }
    $manifestPath = Join-Path $AssetsDir "updater-manifest-$arch.json"
    [System.IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 8), $utf8)
    Copy-Item -LiteralPath $updater -Destination (Join-Path $AssetsDir "SniShaperUpdater-$Version-$arch.exe") -Force
    $count++
    Write-Host "::notice::updater bundle staged: $zipName"
}

if ($count -eq 0) {
    Write-Host "::error::no Inno Setup installer matched under $installerDir"
    throw "no Inno Setup installer matched"
}
Write-Host ("::notice::{0} updater bundle(s) staged in {1}" -f $count, $AssetsDir)
