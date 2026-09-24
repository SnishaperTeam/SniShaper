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

if ($updated -eq $content) {
    Write-Host "[manifest] already at ReleaseChannel=$releaseChannel ReleaseVersion=$releaseVersion IdentityVersion=$identityVersion"
    exit 0
}

# Byte exact write: the manifest is committed back by CI, so no BOM and no
# extra trailing newline may be introduced.
[System.IO.File]::WriteAllText($manifest, $updated, [System.Text.UTF8Encoding]::new($false))
Write-Host "::notice::Manifest updated: ReleaseChannel=$releaseChannel ReleaseVersion=$releaseVersion IdentityVersion=$identityVersion"
