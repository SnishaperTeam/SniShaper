param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$ReleaseVersion,
    [Parameter(Mandatory = $true)][string]$ReleaseChannel,
    [Parameter(Mandatory = $true)][string]$IdentityVersion
)

$ErrorActionPreference = 'Stop'

$manifest = Join-Path $RepoRoot 'Package.appxmanifest'
if (-not (Test-Path $manifest)) {
    Write-Host "::error::Package.appxmanifest not found at $manifest"
    exit 1
}

$releaseVersion = $ReleaseVersion.Trim()
$releaseChannel = $ReleaseChannel.Trim()
$identityVersion = $IdentityVersion.Trim()

if ([string]::IsNullOrWhiteSpace($releaseVersion)) { $releaseVersion = '0.0.0' }
if ([string]::IsNullOrWhiteSpace($releaseChannel)) { $releaseChannel = 'stable' }

# The MSIX Identity version is a four part number and tracks the base
# version only: it never carries the channel, so every prerelease of the
# same base version ships the same package version.
if ($identityVersion -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    Write-Host "::error::identity version '$identityVersion' is not a four part number (expected e.g. 1.29.0.0)"
    exit 1
}

$content = Get-Content -Raw $manifest
$updated = $content
$updated = $updated -replace '<rel:ReleaseChannel>[^<]*</rel:ReleaseChannel>', "<rel:ReleaseChannel>$releaseChannel</rel:ReleaseChannel>"
$updated = $updated -replace '<rel:Version>[^<]*</rel:Version>', "<rel:Version>$releaseVersion</rel:Version>"
$updated = $updated -replace '(?s)(<Identity[^>]*?\sVersion=")[^"]*(")', "`${1}$identityVersion`${2}"

# Parse the rewritten manifest: a silently unmatched replacement would
# otherwise ship a package with stale release metadata.
$doc = New-Object System.Xml.XmlDocument
try {
    $doc.LoadXml($updated)
} catch {
    Write-Host "::error::manifest is no longer valid XML after injection: $($_.Exception.Message)"
    exit 1
}
$ns = [System.Xml.XmlNamespaceManager]::new($doc.NameTable)
$ns.AddNamespace('f', 'http://schemas.microsoft.com/appx/manifest/foundation/windows10')
$ns.AddNamespace('rel', 'http://schemas.snishaper.dev/release')
$relVersionNode = $doc.SelectSingleNode('/f:Package/rel:Version', $ns)
$relChannelNode = $doc.SelectSingleNode('/f:Package/rel:ReleaseChannel', $ns)
$identityNode = $doc.SelectSingleNode('/f:Package/f:Identity', $ns)
if (-not $relVersionNode -or -not $relChannelNode -or -not $identityNode) {
    Write-Host "::error::manifest is missing <rel:Version>, <rel:ReleaseChannel> or <Identity>"
    exit 1
}
if ($relVersionNode.InnerText -ne $releaseVersion) {
    Write-Host "::error::rel:Version not applied (found '$($relVersionNode.InnerText)')"
    exit 1
}
if ($relChannelNode.InnerText -ne $releaseChannel) {
    Write-Host "::error::rel:ReleaseChannel not applied (found '$($relChannelNode.InnerText)')"
    exit 1
}
if ($identityNode.GetAttribute('Version') -ne $identityVersion) {
    Write-Host "::error::Identity Version not applied (found '$($identityNode.GetAttribute('Version'))')"
    exit 1
}

if ($updated -ne $content) {
    # Byte exact write: the manifest is committed back by CI, so no BOM and no
    # extra trailing newline may be introduced.
    [System.IO.File]::WriteAllText($manifest, $updated, [System.Text.UTF8Encoding]::new($false))
    Write-Host "::notice::Manifest updated: ReleaseChannel=$releaseChannel ReleaseVersion=$releaseVersion IdentityVersion=$identityVersion"
} else {
    Write-Host "[manifest] already at ReleaseChannel=$releaseChannel ReleaseVersion=$releaseVersion IdentityVersion=$identityVersion"
}

# ---------------------------------------------------------------------
# docs/SECURITY.md travels with the manifest: every release rewrites the
# Supported Versions table to the line being shipped. A pre-release
# lists its line as best effort, a stable release marks the line as the
# supported one. Same file, same commit, so the security policy can
# never lag the shipped version.
# ---------------------------------------------------------------------
$securityPath = Join-Path $RepoRoot 'docs/SECURITY.md'
if (-not (Test-Path $securityPath)) {
    Write-Host "::error::docs/SECURITY.md not found at $securityPath"
    exit 1
}
if ($releaseVersion -notmatch '^\d+\.\d+') {
    Write-Host "::error::cannot derive a major.minor line from release version '$releaseVersion' for docs/SECURITY.md"
    exit 1
}
$versionParts = $releaseVersion.Split('.')
$line = "$($versionParts[0]).$($versionParts[1])"
# rel_channel is 'stable' for official releases and carries the channel
# suffix (alpha.N / beta.N / rc.N) for pre-releases.
$isPrerelease = ($releaseChannel -ne 'stable')
$versionCell = if ($isPrerelease) { "$line.x pre-releases (alpha / beta / rc)" } else { "$line.x (latest stable line)" }
$supportedCell = if ($isPrerelease) { '✅ (best effort)' } else { '✅' }

$col1 = [Math]::Max('Version'.Length, $versionCell.Length)
$col2 = [Math]::Max('Supported'.Length, $supportedCell.Length)
$tableRows = @(
    @('Version'.PadRight($col1), 'Supported'.PadRight($col2)),
    @(('-' * $col1), ('-' * $col2)),
    @($versionCell.PadRight($col1), $supportedCell.PadRight($col2))
)
$table = ($tableRows | ForEach-Object { "| $($_[0]) | $($_[1]) |" }) -join "`n"

$security = [System.IO.File]::ReadAllText($securityPath)
if ($security.IndexOf('## Supported Versions') -lt 0) {
    Write-Host "::error::docs/SECURITY.md has no '## Supported Versions' section"
    exit 1
}
# Replace the first markdown table after the section heading (the only
# table in the file), keeping the file's own newline style.
$newline = if ($security.Contains("`r`n")) { "`r`n" } else { "`n" }
$tableRegex = [regex]'(?m)^(?:\|[^\r\n]*\r?\n)+'
$tableMatch = $tableRegex.Match($security, $security.IndexOf('## Supported Versions'))
if (-not $tableMatch.Success) {
    Write-Host "::error::no supported-versions table found in docs/SECURITY.md"
    exit 1
}
$newSecurity = $security.Substring(0, $tableMatch.Index) + $table.Replace("`n", $newline) + $newline + $security.Substring($tableMatch.Index + $tableMatch.Length)
if ($newSecurity -notmatch [regex]::Escape($versionCell)) {
    Write-Host "::error::docs/SECURITY.md table rewrite did not apply"
    exit 1
}
if ($newSecurity -ne $security) {
    [System.IO.File]::WriteAllText($securityPath, $newSecurity, [System.Text.UTF8Encoding]::new($false))
    Write-Host "::notice::docs/SECURITY.md updated: '$versionCell' -> '$supportedCell'"
} else {
    Write-Host "[security] docs/SECURITY.md already at '$versionCell'"
}
