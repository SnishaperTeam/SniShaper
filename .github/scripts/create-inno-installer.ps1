param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $false)][string]$Suffix = "",
    [Parameter(Mandatory = $false)][string[]]$Arch = @()
)

$ErrorActionPreference = 'Stop'

Set-Location $RepoRoot

$displayVersion = if ($Suffix) { "$Version-$Suffix" } else { "$Version" }
Write-Host "[inno] Version=$Version Suffix=$Suffix Display=$displayVersion"

$licenseFile = Join-Path $RepoRoot 'LICENSE'
if (-not (Test-Path $licenseFile)) {
    Write-Host "::error::LICENSE not found at repo root"
    exit 1
}

$iscc = 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
if (-not (Test-Path $iscc)) {
    $found = Get-ChildItem -Path 'C:\Program Files*\Inno Setup*' -Filter 'ISCC.exe' -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($found) { $iscc = $found.FullName }
}
if (-not (Test-Path $iscc)) {
    Write-Host "::error::ISCC.exe not found (Inno Setup install failed)"
    exit 1
}

# Simplified Chinese messages file ships with the repo (Inno Setup's own
# install only provides ChineseSimplified.isl, not the _2 variant). Copy it
# into the Inno Languages dir so the compiler:Languages\... reference works.
$innoDir = Split-Path -Parent $iscc
$langsDir = Join-Path $innoDir 'Languages'
New-Item -ItemType Directory -Path $langsDir -Force | Out-Null
$zhIsl = Join-Path $RepoRoot '.github\ChineseSimplified_2.isl'
if (-not (Test-Path $zhIsl)) {
    Write-Host "::error::.github/ChineseSimplified_2.isl not found"
    exit 1
}
Copy-Item -Path $zhIsl -Destination (Join-Path $langsDir 'ChineseSimplified_2.isl') -Force

# Windows GUI payload lives at build/bin/gui/Windows/<arch>/. Every
# architecture the build scripts produced gets its own installer, so the
# release carries the same set as the portable archives.
$requested = if ($Arch.Count -gt 0) { $Arch } else { @('x64', 'x86', 'arm64') }
$built = @()
foreach ($candidate in $requested) {
    $dir = Join-Path $RepoRoot "build/bin/gui/Windows/$candidate"
    if (Test-Path (Join-Path $dir 'snishaper.exe')) {
        $built += [pscustomobject]@{ Arch = $candidate; Dir = $dir }
    } else {
        Write-Host "::warning::no Windows GUI payload for '$candidate' (expected build/bin/gui/Windows/$candidate/snishaper.exe); skipping its installer"
    }
}
if ($built.Count -eq 0) {
    Write-Host "::error::no Windows GUI payload found (expected build/bin/gui/Windows/<arch>/snishaper.exe)"
    exit 1
}

# Inno Setup architecture tokens. x64 uses x64compatible so the installer also
# runs on ARM64 Windows via emulation; 32-bit x86 must drop the 64-bit mode
# flag entirely or ISCC refuses the script.
$archTokens = @{
    'x64'   = @{ Allowed = 'x64compatible'; Mode64 = 'x64compatible' }
    'arm64' = @{ Allowed = 'arm64';         Mode64 = 'arm64' }
    'x86'   = @{ Allowed = 'x86';           Mode64 = '' }
}

$outDir = Join-Path $RepoRoot 'installer'
New-Item -ItemType Directory -Path $outDir -Force | Out-Null

$issTemplate = @'
; Inno Setup script generated for SniShaper CI builds
#define MyAppName "Snishaper"
#define MyAppVersion "__VERSION__"
#define MyAppPublisher "SnishaperTeam And JetCPPTeam"
#define MyAppURL "https://jetcpp.ccwu.cc/"
#define MyAppExeName "snishaper.exe"

[Setup]
AppId={{__APPID__}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
UninstallDisplayIcon={app}\{#MyAppExeName}
ArchitecturesAllowed=__ARCHALLOWED__
ArchitecturesInstallIn64BitMode=__ARCHMODE64__
DisableProgramGroupPage=yes
LicenseFile="__LICENSE__"
PrivilegesRequiredOverridesAllowed=dialog
OutputDir="__OUTDIR__"
OutputBaseFilename="__OUTNAME__"
SolidCompression=yes
WizardStyle=modern polar

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
__ZH_LANG__
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "__BINDIR__\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "__BINDIR__\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
'@

# Distinct AppId per architecture: Inno treats a shared AppId as the same
# product, so an x86 install would collide with an existing x64 one.
$appIds = @{
    'x64'   = '3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D4'
    'x86'   = '3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D5'
    'arm64' = '3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D6'
}

$zhLangLine = 'Name: "chinesesimplified_2"; MessagesFile: "compiler:Languages\ChineseSimplified_2.isl"'
$produced = @()

foreach ($entry in $built) {
    $setupArch = $entry.Arch
    $binDir = $entry.Dir
    $outName = "Snishaper-$displayVersion-${setupArch}Setup"

    if (-not $archTokens.ContainsKey($setupArch)) {
        Write-Host "::error::unsupported installer architecture '$setupArch'"
        exit 1
    }

    $iss = $issTemplate
    $iss = $iss.Replace('__VERSION__', $Version)
    $iss = $iss.Replace('__LICENSE__', $licenseFile.Replace('\', '\\'))
    $iss = $iss.Replace('__OUTDIR__', $outDir)
    $iss = $iss.Replace('__OUTNAME__', $outName)
    $iss = $iss.Replace('__BINDIR__', $binDir)
    $iss = $iss.Replace('__ARCHALLOWED__', $archTokens[$setupArch].Allowed)
    $iss = $iss.Replace('__ARCHMODE64__', $archTokens[$setupArch].Mode64)
    $iss = $iss.Replace('__APPID__', $appIds[$setupArch])
    $iss = $iss.Replace('__ZH_LANG__', $zhLangLine)

    $issPath = Join-Path $RepoRoot "installer-$setupArch.iss"
    [System.IO.File]::WriteAllText($issPath, $iss, [System.Text.Encoding]::UTF8)
    Write-Host "[inno] .iss written to $issPath (arch=$setupArch)"

    Write-Host "::group::ISCC compile $issPath"
    & $iscc /Qp $issPath
    if ($LASTEXITCODE -ne 0) {
        Write-Host "::error::Inno Setup compile failed for $setupArch (exit $LASTEXITCODE)"
        exit 1
    }
    Write-Host "::endgroup::"

    $setupExe = Get-Item -Path (Join-Path $outDir "$outName.exe") -ErrorAction SilentlyContinue
    if (-not $setupExe) {
        Write-Host "::error::no Setup exe produced for $setupArch"
        exit 1
    }
    Write-Host "::notice::Inno Setup installer ready: $($setupExe.FullName)"
    $produced += $setupArch
}

Write-Host "::notice::installers produced: $($produced -join ', ')"
exit 0
