param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Channel,
    [Parameter(Mandatory = $false)][string]$PrereleaseSuffix = "",
    [Parameter(Mandatory = $false)][string]$PreviousTag = "",
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [Parameter(Mandatory = $false)][string]$OllamaUrl = "http://127.0.0.1:11434",
    [Parameter(Mandatory = $false)][string]$OllamaModel = "qwen3.5:2b",
    [Parameter(Mandatory = $false)][string]$LlmApiKey = "",
    [Parameter(Mandatory = $false)][string]$LlmModel = "gpt-4o-mini",
    [Parameter(Mandatory = $false)][string]$LlmBaseUrl = "https://api.openai.com/v1",
    [Parameter(Mandatory = $false)][int]$LlmMaxCommits = 80
)

$ErrorActionPreference = 'Stop'
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
} catch {
    Write-Host "[release-notes] warning: failed to set UTF-8 console encoding"
}
Set-Location $RepoRoot

$channelLabel = switch ($Channel) {
    'beta' { 'Beta' }
    'alpha' { 'Alpha' }
    'rc' { 'Release Candidate' }
    default { 'Release' }
}

Write-Host "[release-notes] Channel=$Channel Version=$Version Suffix=$PrereleaseSuffix PrevTag=$PreviousTag"

if ($PrereleaseSuffix) {
    $displayVersion = "$Version-$PrereleaseSuffix"
} else {
    $displayVersion = $Version
}

if ($PreviousTag) {
    $range = "$PreviousTag..HEAD"
    $rangeDesc = "Changes since the previous stable release tag $PreviousTag"
} else {
    $range = "HEAD"
    $rangeDesc = "First release: all commits included"
}
Write-Host "[release-notes] Commit range: $range"

$gitLog = git log --pretty=format:"%h%x09%s%x09%an%x09%ae" $range 2>&1
if ($LASTEXITCODE -ne 0) {
    throw "git log failed: $gitLog"
}

$lines = ($gitLog | Out-String) -split "`r?`n"

function Get-CommitGroup {
    param([string]$Subject)
    if ($Subject -match '^feat(\(.+\))?:') { return 'feat' }
    if ($Subject -match '^fix(\(.+\))?:') { return 'fix' }
    if ($Subject -match '^perf(\(.+\))?:') { return 'perf' }
    if ($Subject -match '^refactor(\(.+\))?:') { return 'refactor' }
    if ($Subject -match '^docs(\(.+\))?:') { return 'docs' }
    if ($Subject -match '^(build|ci)(\(.+\))?:') { return 'build' }
    if ($Subject -match '^test(\(.+\))?:') { return 'test' }
    return 'other'
}

function Test-SummaryShape {
    param([string]$Text)
    if ([string]::IsNullOrWhiteSpace($Text)) {
        Write-Host "::warning::summary rejected: empty output"
        return $false
    }
    $headings = ([regex]::Matches($Text, '(?m)^#{2,3} \S')).Count
    $bullets = ([regex]::Matches($Text, '(?m)^\s*[-*] \S')).Count
    if ($headings -lt 1) {
        Write-Host "::warning::summary rejected: no Markdown category heading"
        return $false
    }
    if ($bullets -lt 3) {
        Write-Host "::warning::summary rejected: only $bullets bullet(s)"
        return $false
    }
    if ($bullets -gt 25) {
        Write-Host "::warning::summary rejected: $bullets bullets exceed the 25 limit"
        return $false
    }
    return $true
}

$groups = @{
    feat     = @()
    fix      = @()
    docs     = @()
    refactor = @()
    perf     = @()
    build    = @()
    test     = @()
    other    = @()
}
$authors = @{}
$totalCommits = 0
$commitList = @()

foreach ($line in $lines) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    $parts = $line -split "`t"
    if ($parts.Count -lt 4) { continue }
    $hash = $parts[0]
    $subject = $parts[1]
    $author = $parts[2]
    $email = $parts[3]
    $totalCommits++

    $key = Get-CommitGroup -Subject $subject

    $groups[$key] += [PSCustomObject]@{ Hash = $hash; Subject = $subject }
    $commitList += [PSCustomObject]@{ Hash = $hash; Subject = $subject }
    if (-not $authors.ContainsKey($email)) {
        $authors[$email] = @{ Name = $author; Count = 0 }
    }
    $authors[$email].Count++
}

# ---------- LLM summarization (Ollama first, then external API) ----------
$llmSummary = $null

$noisePattern = '^(Merge |Bump |chore\(deps|chore\(dependabot)'
$categoryOrder = @('feat', 'fix', 'perf', 'refactor', 'docs', 'build', 'test', 'other')
$categoryCaps = @{ feat = 12; fix = 15; perf = 6; refactor = 8; docs = 4; build = 6; test = 3; other = 10 }
$seenSubjects = @{}
$digestBuckets = @{}
foreach ($k in $categoryOrder) { $digestBuckets[$k] = @() }
$droppedNoise = 0
$droppedDuplicate = 0
foreach ($c in $commitList) {
    if ($c.Subject -match $noisePattern) { $droppedNoise++; continue }
    $norm = $c.Subject.Trim().ToLowerInvariant()
    if ($seenSubjects.ContainsKey($norm)) { $droppedDuplicate++; continue }
    $seenSubjects[$norm] = $true
    $digestBuckets[(Get-CommitGroup -Subject $c.Subject)] += $c.Subject
}

$digestLines = @()
$budget = $LlmMaxCommits
$fedCount = 0
foreach ($k in $categoryOrder) {
    $items = $digestBuckets[$k]
    if ($items.Count -eq 0) { continue }
    if ($budget -le 0) { continue }
    $take = [Math]::Min($items.Count, [int]$categoryCaps[$k])
    if ($take -gt $budget) { $take = $budget }
    $digestLines += "### $k ($($items.Count))"
    foreach ($s in ($items | Select-Object -First $take)) { $digestLines += "- $s" }
    $budget -= $take
    $fedCount += $take
}
$commitText = $digestLines -join "`n"
Write-Host "[release-notes] LLM input: $totalCommits commits -> $fedCount selected ($($digestLines.Count - $fedCount) group headings); dropped $droppedNoise automated/merge and $droppedDuplicate duplicate subject(s)"

$systemPrompt = @"
You are a senior technical writer producing formal, rigorous English release notes for an open-source software project.

LANGUAGE CONSTRAINT (highest priority):
- The ENTIRE output MUST be written in English. Chinese, Japanese, or any other language is strictly forbidden.
- All section headings, bullet points, and sentences must be English.
- If you are unsure how to express something in English, choose simpler English phrasing rather than switching languages.

Writing requirements:
1. Use formal, precise, professional English. Be objective and factual; avoid marketing tone, casual phrasing, and filler.
2. Be detailed and concrete: for each category, explain what changed, why it changed, and its impact on users or the system.
3. Structure the output clearly with Markdown headings (### and below), lists, and bullets.
4. Strictly forbid any emoji or emoticons. Do not output commit hashes.
5. Output only the release-notes body. No preamble, postscript, or explanatory text.

Hard limits (violating these makes the notes unusable):
6. At most 12 bullet points in total, across all sections combined.
7. The first line MUST be a Markdown heading (### ...). Never open with a sentence such as "Here is ...", a greeting, or any other preamble.
8. Rewrite each change as a user-facing statement. Never copy a commit subject verbatim and never emit a raw list of commit titles.
9. Skip sections that have no changes.

Expected output shape (English only):
### New Features
- Feature A: what it does and its impact.
- Feature B: ...
### Bug Fixes
- Fix A: what was broken and how it is resolved.
"@

$userPrompt = @"
Write the official English release notes for this version of SniShaper (a Windows local proxy tool), based on the grouped change list below.

The list is pre-grouped by conventional-commit type: feat = New Features, fix = Bug Fixes, perf = Performance Improvements, refactor = Refactoring, docs = Documentation, build = Build & CI, test = Tests, other = Other. Automated dependency bumps and merge commits have already been removed, so every entry is a real change. The number in each heading is the total count for that type; only a representative subset is listed.

LANGUAGE CONSTRAINT (highest priority):
- The ENTIRE output MUST be entirely in English. Do not use Chinese or any other language anywhere in the output.
- Category names MUST be English, e.g. New Features, Bug Fixes, Performance Improvements, Refactoring, Documentation, Build & CI, Tests, Other.

Writing requirements:
1. Organize the content by change type, e.g.: New Features, Bug Fixes, Performance Improvements, Refactoring, Documentation, Build & CI, Tests, Other.
2. For each type, describe the core changes in detail: what was modified, why, and the impact on users or the system. Use one or more concise bullet points per item, and at most 12 bullet points in total.
3. If a change touches multiple modules (proxy core, TUN, frontend UI, build scripts, etc.), break them out per module.
4. Minor changes such as dependency bumps, formatting, or merges may be condensed into a single brief note.
5. Write in formal, rigorous English. Strictly forbid emoji. Do not output commit hashes, and do not copy commit subjects verbatim.

Grouped change list:
$commitText
"@

# --- Priority 1: local Ollama ---
$ollamaAvailable = $false
try {
    Write-Host "[release-notes] Checking Ollama at $OllamaUrl"
    $tagsResp = Invoke-RestMethod -Uri "$OllamaUrl/api/tags" -Method Get -TimeoutSec 10
    $ollamaAvailable = $true
    Write-Host "[release-notes] Ollama reachable, installed models: $($tagsResp.models.model -join ', ')"
} catch {
    Write-Host "::warning::Ollama not available ($($_.Exception.Message))"
}

if ($ollamaAvailable) {
    Write-Host "[release-notes] Generating summary via local Ollama model=$OllamaModel"
    $ollamaBody = @{
        model    = $OllamaModel
        messages = @(
            @{ role = 'system'; content = $systemPrompt },
            @{ role = 'user'; content = $userPrompt }
        )
        stream   = $false
        # Qwen3+ models enable thinking mode by default; the reasoning
        # goes into message.thinking while message.content stays empty.
        # Disable it so the final answer is returned in message.content.
        think    = $false
        options  = @{ temperature = 0.3; num_predict = 1200 }
    } | ConvertTo-Json -Depth 6
    try {
        $resp = Invoke-RestMethod -Uri "$OllamaUrl/api/chat" -Method Post -ContentType 'application/json; charset=utf-8' -Body $ollamaBody -TimeoutSec 300
        if ($resp.message -and $resp.message.content) {
            $candidate = $resp.message.content.Trim()
            if (Test-SummaryShape -Text $candidate) {
                $llmSummary = $candidate
                Write-Host "[release-notes] Ollama summary generated ($($llmSummary.Length) chars, shape check passed)"
            } else {
                Write-Host "::warning::Ollama summary failed the shape check (model=$OllamaModel). Falling back."
            }
        } elseif ($resp.message -and $resp.message.thinking) {
            # thinking present but no final content - treat as failure
            Write-Host "::warning::Ollama returned thinking but empty content (model=$OllamaModel). Falling back."
        } else {
            Write-Host "::warning::Ollama returned empty response (model=$OllamaModel). Falling back."
        }
    } catch {
        Write-Host "::warning::Ollama inference failed: $($_.Exception.Message)"
    }
}

# --- Priority 2: external OpenAI-compatible API ---
if (-not $llmSummary -and -not [string]::IsNullOrEmpty($LlmApiKey)) {
    Write-Host "[release-notes] LLM summarization via external API (model=$LlmModel, base=$LlmBaseUrl, commits=$fedCount)"
    $body = @{
        model    = $LlmModel
        messages = @(
            @{ role = 'system'; content = $systemPrompt },
            @{ role = 'user'; content = $userPrompt }
        )
        temperature = 0.3
        max_tokens  = 1200
    } | ConvertTo-Json -Depth 6

    try {
        $uri = $LlmBaseUrl.TrimEnd('/')
        if (-not $uri.EndsWith('/chat/completions')) {
            $uri += '/chat/completions'
        }
        $headers = @{ Authorization = "Bearer $LlmApiKey" }
        Write-Host "[release-notes] POST $uri"
        $resp = Invoke-RestMethod -Uri $uri -Method Post -Headers $headers -ContentType 'application/json; charset=utf-8' -Body $body -TimeoutSec 120
        if ($resp.choices -and $resp.choices.Count -gt 0) {
            $candidate = $resp.choices[0].message.content.Trim()
            if (Test-SummaryShape -Text $candidate) {
                $llmSummary = $candidate
                Write-Host "[release-notes] LLM summary generated ($($llmSummary.Length) chars, shape check passed)"
            } else {
                Write-Host "::warning::LLM summary failed the shape check (model=$LlmModel). Falling back to categorized list."
            }
        } else {
            Write-Host "::warning::LLM response missing choices, falling back to categorized list"
        }
    } catch {
        Write-Host "::warning::LLM summarization failed: $($_.Exception.Message). Falling back to categorized list."
        $llmSummary = $null
    }
}

if (-not $llmSummary) {
    Write-Host "[release-notes] Using categorized commit list (no LLM summary accepted)"
}

$sb = New-Object System.Text.StringBuilder

if ($PrereleaseSuffix) {
    [void]$sb.AppendLine("# SniShaper $displayVersion ($channelLabel)")
    [void]$sb.AppendLine("")
    [void]$sb.AppendLine("> Prerelease version: This release may be unstable; some features may be removed or reworked. It does not represent the final version.")
    [void]$sb.AppendLine("")
    [void]$sb.AppendLine("---")
    [void]$sb.AppendLine("")
} else {
    [void]$sb.AppendLine("# SniShaper $displayVersion")
    [void]$sb.AppendLine("")
    [void]$sb.AppendLine("---")
    [void]$sb.AppendLine("")
}

[void]$sb.AppendLine("## Version Information")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("- Build time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')")
[void]$sb.AppendLine("- Version source: Package.appxmanifest")
[void]$sb.AppendLine("- Commit range: $rangeDesc")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("---")
[void]$sb.AppendLine("")

[void]$sb.AppendLine("## Changes ($totalCommits commits)")
[void]$sb.AppendLine("")

if ($llmSummary) {
    [void]$sb.AppendLine($llmSummary)
    [void]$sb.AppendLine("")
} else {
    # Fallback: grouped commit list, cap each category to a few representative entries
    $sectionMap = @{
        feat     = 'New Features'
        fix      = 'Bug Fixes'
        perf     = 'Performance Improvements'
        refactor = 'Refactoring'
        docs     = 'Documentation'
        build    = 'Build & CI'
        test     = 'Tests'
        other    = 'Other'
    }
    foreach ($key in @('feat', 'fix', 'perf', 'refactor', 'docs', 'build', 'test', 'other')) {
        $items = $groups[$key]
        if ($items.Count -eq 0) { continue }
        [void]$sb.AppendLine("### $($sectionMap[$key]) ($($items.Count))")
        [void]$sb.AppendLine("")
        $shown = $items | Select-Object -First 8
        foreach ($item in $shown) {
            [void]$sb.AppendLine("- $($item.Hash) $($item.Subject)")
        }
        if ($items.Count -gt 8) {
            [void]$sb.AppendLine("- ... and $($items.Count - 8) more commits")
        }
        [void]$sb.AppendLine("")
    }
}

[void]$sb.AppendLine("---")
[void]$sb.AppendLine("")

[void]$sb.AppendLine("## Contributors ($($authors.Count))")
[void]$sb.AppendLine("")
$sorted = $authors.GetEnumerator() | Sort-Object { $_.Value.Count } -Descending
foreach ($a in $sorted) {
    $commitWord = if ($a.Value.Count -gt 1) { 'commits' } else { 'commit' }
    [void]$sb.AppendLine("- $($a.Value.Name) <$($a.Key)>: $($a.Value.Count) $commitWord")
}
[void]$sb.AppendLine("")

[System.IO.File]::WriteAllText($OutputPath, $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Host "[release-notes] Release notes written to $OutputPath"
