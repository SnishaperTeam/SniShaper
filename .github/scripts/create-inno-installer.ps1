param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $false)][string]$Suffix = ""
)

$ErrorActionPreference = 'Stop'

Set-Location $RepoRoot

$displayVersion = if ($Suffix) { "$Version-$Suffix" } else { $Version }
Write-Host "[inno] Version=$Version Suffix=$Suffix Display=$displayVersion"

$licenseFile = Join-Path $RepoRoot 'LICENSE'
if (-not (Test-Path $licenseFile)) {
    Write-Host "::error::LICENSE not found at repo root"
    exit 1
}

$binDir = Join-Path $RepoRoot 'build/bin'
if (-not (Test-Path (Join-Path $binDir 'snishaper.exe'))) {
    Write-Host "::error::build/bin/snishaper.exe not found"
    exit 1
}

$outDir = Join-Path $RepoRoot 'installer'
New-Item -ItemType Directory -Path $outDir -Force | Out-Null
$outName = "Snishaper-$displayVersion-x64Setup"

$iss = @'
; Inno Setup script generated for SniShaper CI builds
#define MyAppName "Snishaper"
#define MyAppVersion "__VERSION__"
#define MyAppPublisher "SnishaperTeam And JetCPPTeam"
#define MyAppURL "https://jetcpp.dpdns.org/snishaper/index.html"
#define MyAppExeName "snishaper.exe"

[Setup]
AppId={{3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D4}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
UninstallDisplayIcon={app}\{#MyAppExeName}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
DisableProgramGroupPage=yes
LicenseFile="__LICENSE__"
PrivilegesRequiredOverridesAllowed=dialog
OutputDir="__OUTDIR__"
OutputBaseFilename="__OUTNAME__"
SolidCompression=yes
WizardStyle=modern polar

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "chinesesimplified_2"; MessagesFile: "compiler:Languages\ChineseSimplified_2.isl"
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

$iss = $iss.Replace('__VERSION__', $Version)
$iss = $iss.Replace('__LICENSE__', $licenseFile.Replace('\', '\\'))
$iss = $iss.Replace('__OUTDIR__', $outDir)
$iss = $iss.Replace('__OUTNAME__', $outName)
$iss = $iss.Replace('__BINDIR__', $binDir)

$issPath = Join-Path $RepoRoot 'installer.iss'
[System.IO.File]::WriteAllText($issPath, $iss, [System.Text.Encoding]::UTF8)
Write-Host "[inno] .iss written to $issPath"

$iscc = 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
if (-not (Test-Path $iscc)) {
    $found = Get-ChildItem -Path 'C:\Program Files*\Inno Setup*' -Filter 'ISCC.exe' -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($found) { $iscc = $found.FullName }
}
if (-not (Test-Path $iscc)) {
    Write-Host "::error::ISCC.exe not found (Inno Setup install failed)"
    exit 1
}

Write-Host "::group::ISCC compile $issPath"
& $iscc /Qp $issPath
if ($LASTEXITCODE -ne 0) {
    Write-Host "::error::Inno Setup compile failed (exit $LASTEXITCODE)"
    exit 1
}
Write-Host "::endgroup::"

$setupExe = Get-ChildItem -Path $outDir -Filter '*.exe' | Select-Object -First 1
if (-not $setupExe) {
    Write-Host "::error::no Setup exe produced"
    exit 1
}
Write-Host "::notice::Inno Setup installer ready: $($setupExe.FullName)"
exit 0
