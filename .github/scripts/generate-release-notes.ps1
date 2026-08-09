param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Channel,
    [Parameter(Mandatory = $false)][string]$PrereleaseSuffix = "",
    [Parameter(Mandatory = $false)][string]$PreviousTag = "",
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [Parameter(Mandatory = $false)][string]$OllamaUrl = "http://127.0.0.1:11434",
    [Parameter(Mandatory = $false)][string]$OllamaModel = "qwen3.5:0.8b",
    [Parameter(Mandatory = $false)][string]$LlmApiKey = "",
    [Parameter(Mandatory = $false)][string]$LlmModel = "gpt-4o-mini",
    [Parameter(Mandatory = $false)][string]$LlmBaseUrl = "https://api.openai.com/v1",
    [Parameter(Mandatory = $false)][int]$LlmMaxCommits = 400
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
    throw "git log 失败: $gitLog"
}

$lines = ($gitLog | Out-String) -split "`r?`n"

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

    $key = 'other'
    if ($subject -match '^feat(\(.+\))?:') { $key = 'feat' }
    elseif ($subject -match '^fix(\(.+\))?:') { $key = 'fix' }
    elseif ($subject -match '^docs(\(.+\))?:') { $key = 'docs' }
    elseif ($subject -match '^refactor(\(.+\))?:') { $key = 'refactor' }
    elseif ($subject -match '^perf(\(.+\))?:') { $key = 'perf' }
    elseif ($subject -match '^(build|ci)(\(.+\))?:') { $key = 'build' }
    elseif ($subject -match '^test(\(.+\))?:') { $key = 'test' }

    $groups[$key] += [PSCustomObject]@{ Hash = $hash; Subject = $subject }
    $commitList += [PSCustomObject]@{ Hash = $hash; Subject = $subject }
    if (-not $authors.ContainsKey($email)) {
        $authors[$email] = @{ Name = $author; Count = 0 }
    }
    $authors[$email].Count++
}

# ---------- LLM summarization (Ollama first, then external API) ----------
$llmSummary = $null

$batch = $commitList | Select-Object -First $LlmMaxCommits
$commitText = ($batch | ForEach-Object { "- $($_.Hash) $($_.Subject)" }) -join "`n"

$systemPrompt = @"
You are a senior technical writer producing formal, rigorous English release notes for an open-source software project.

Writing requirements:
1. Use formal, precise, professional English. Be objective and factual; avoid marketing tone, casual phrasing, and filler.
2. Be detailed and concrete: for each category, explain what changed, why it changed, and its impact on users or the system.
3. Structure the output clearly with Markdown headings (### and below), lists, and bullets.
4. Strictly forbid any emoji or emoticons. Do not output commit hashes.
5. Output only the release-notes body. No preamble, postscript, or explanatory text.
"@

$userPrompt = @"
Write the official English release notes for this version of SniShaper (a Windows local proxy tool), based on the commit message list below.

Writing requirements:
1. Organize the content by change type, e.g.: New Features, Bug Fixes, Performance Improvements, Refactoring, Documentation, Build & CI, Tests, Other.
2. For each type, describe the core changes in detail: what was modified, why, and the impact on users or the system. Use one or more concise bullet points per item.
3. If a change touches multiple modules (proxy core, TUN, frontend UI, build scripts, etc.), break them out per module.
4. Minor changes such as dependency bumps, formatting, or merges may be condensed into a single brief note.
5. Write in formal, rigorous English. Strictly forbid emoji. Do not output commit hashes.

Commit log:
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
    } | ConvertTo-Json -Depth 6
    try {
        $resp = Invoke-RestMethod -Uri "$OllamaUrl/api/chat" -Method Post -ContentType 'application/json; charset=utf-8' -Body $ollamaBody -TimeoutSec 300
        if ($resp.message -and $resp.message.content) {
            $llmSummary = $resp.message.content.Trim()
            Write-Host "[release-notes] Ollama summary generated ($($llmSummary.Length) chars)"
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
    Write-Host "[release-notes] LLM summarization via external API (model=$LlmModel, base=$LlmBaseUrl, commits=$($commitList.Count))"
    $body = @{
        model    = $LlmModel
        messages = @(
            @{ role = 'system'; content = $systemPrompt },
            @{ role = 'user'; content = $userPrompt }
        )
        temperature = 0.3
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
            $llmSummary = $resp.choices[0].message.content.Trim()
            Write-Host "[release-notes] LLM summary generated ($($llmSummary.Length) chars)"
        } else {
            Write-Host "::warning::LLM response missing choices, falling back to categorized list"
        }
    } catch {
        Write-Host "::warning::LLM summarization failed: $($_.Exception.Message). Falling back to categorized list."
        $llmSummary = $null
    }
}

if (-not $llmSummary) {
    Write-Host "[release-notes] Using categorized commit list (Ollama and/or external LLM unavailable)"
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
Write-Host "[release-notes] 已生成 $OutputPath"
