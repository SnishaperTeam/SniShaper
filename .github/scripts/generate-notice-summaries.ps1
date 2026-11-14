# Generates localized, plain-text in-app announcement summaries from the
# release notes via one LLM call. Writes the raw Markdown reply (## zh /
# ## en / ## ru sections) to OutputPath only when it passed validation;
# always exits 0 so a failure here only means the announcement falls back
# to the raw changes text.
param(
    [Parameter(Mandatory = $true)][string]$ReleaseNotesPath,
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [Parameter(Mandatory = $false)][string]$OllamaUrl = "http://127.0.0.1:11434",
    [Parameter(Mandatory = $false)][string]$OllamaModel = "qwen3.5:2b",
    [Parameter(Mandatory = $false)][string]$LlmApiKey = "",
    [Parameter(Mandatory = $false)][ValidateSet('external', 'ollama')][string]$LlmPriority = "external",
    [Parameter(Mandatory = $false)][string]$LlmModel = "poolside/laguna-xs-2.1",
    [Parameter(Mandatory = $false)][string]$LlmBaseUrl = "https://integrate.api.nvidia.com/v1",
    [Parameter(Mandatory = $false)][int]$LlmMaxTokens = 900
)

if (-not $LlmApiKey -and $env:LLM_API_KEY) { $LlmApiKey = $env:LLM_API_KEY }
if (-not $PSBoundParameters.ContainsKey('LlmModel') -and $env:LLM_MODEL) { $LlmModel = $env:LLM_MODEL }
if (-not $PSBoundParameters.ContainsKey('LlmBaseUrl') -and $env:LLM_BASE_URL) { $LlmBaseUrl = $env:LLM_BASE_URL }
if (-not $PSBoundParameters.ContainsKey('OllamaModel') -and $env:OLLAMA_MODEL) { $OllamaModel = $env:OLLAMA_MODEL }
if (-not $PSBoundParameters.ContainsKey('OllamaUrl') -and $env:OLLAMA_URL) { $OllamaUrl = $env:OLLAMA_URL }

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $ReleaseNotesPath)) {
    Write-Host "[notice-summaries] release notes not found: $ReleaseNotesPath"
    exit 0
}

$notes = [System.IO.File]::ReadAllText($ReleaseNotesPath, [System.Text.Encoding]::UTF8)
$changes = ''
$match = [regex]::Match($notes, '(?s)##\s*Changes\s*\r?\n(.*?)(?=\r?\n##\s|\z)')
if ($match.Success) { $changes = $match.Groups[1].Value.Trim() }
if (-not $changes) { $changes = $notes.Trim() }
if ($changes.Length -gt 6000) { $changes = $changes.Substring(0, 6000).TrimEnd() + '...' }
if (-not $changes) {
    Write-Host "[notice-summaries] release notes carry no changes text"
    exit 0
}

$systemPrompt = @"
You write short in-app release announcements for a desktop application.
Return ONLY Markdown in exactly this structure, with nothing before or after it:

## zh
<2 to 4 short sentences in Simplified Chinese>

## en
<2 to 4 short sentences in English>

## ru
<2 to 4 short sentences in Russian>

Rules for every summary:
- Plain text only: no Markdown syntax, no bullet markers, no emoji, no links, no version numbers.
- Summarize the most important user-facing changes.
- No preamble such as ""Here is""; the three headings must be the entire output.
"@

$userPrompt = @"
Summarize the following release notes of SniShaper (a Windows local proxy tool) into the three localized announcement texts as specified.

Release notes:
$changes
"@

function Test-SummaryMd {
    param([string]$Text)
    foreach ($lang in @('zh', 'en', 'ru')) {
        # (?s) lets multi-line content match; the lookahead tolerates blank
        # lines between sections (LLMs often emit "...\n\n## en").
        $m = [regex]::Match($Text, "(?sim)^##\s*$lang\s*\r?\n(.*?)(?=\r?\n\s*##\s|\z)")
        if (-not $m.Success) { return $false }
        $v = $m.Groups[1].Value.Trim()
        if (-not $v -or $v.Length -gt 600 -or $v -match '#|```') { return $false }
    }
    return $true
}

$summaryMd = $null

# --- Preferred: OpenAI-compatible API ---
if ($LlmApiKey) {
    Write-Host "[notice-summaries] LLM summarization via API (model=$LlmModel, base=$LlmBaseUrl)"
    $body = @{
        model       = $LlmModel
        messages    = @(
            @{ role = 'system'; content = $systemPrompt },
            @{ role = 'user'; content = $userPrompt }
        )
        temperature = 0.3
        max_tokens  = $LlmMaxTokens
    }
    if ($LlmBaseUrl -match 'bigmodel\.cn') {
        $body.thinking = @{ type = 'disabled' }
    }
    $bodyJson = $body | ConvertTo-Json -Depth 6
    $uri = $LlmBaseUrl.TrimEnd('/')
    if (-not $uri.EndsWith('/chat/completions')) { $uri += '/chat/completions' }
    $headers = @{ Authorization = "Bearer $LlmApiKey" }

    $maxAttempts = 3
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        try {
            $resp = Invoke-RestMethod -Uri $uri -Method Post -Headers $headers -ContentType 'application/json; charset=utf-8' -Body $bodyJson -TimeoutSec 120
            $content = $null
            if ($resp.choices -and $resp.choices.Count -gt 0 -and $resp.choices[0].message) {
                $content = $resp.choices[0].message.content
            }
            if ($content) {
                if (Test-SummaryMd -Text $content) {
                    $summaryMd = $content.Trim()
                    Write-Host "[notice-summaries] localized summaries generated (attempt $attempt)"
                    break
                }
                Write-Host "::warning::LLM response lacks valid ## zh / ## en / ## ru sections (attempt $attempt)"
            } else {
                Write-Host "::warning::API response missing content (attempt $attempt)"
            }
        } catch {
            Write-Host "::warning::LLM request failed (attempt $attempt/$maxAttempts): $($_.Exception.Message)"
            if ($attempt -lt $maxAttempts) { Start-Sleep -Seconds (10 * $attempt) }
        }
    }
}

# --- Fallback: local Ollama (still running from the release-notes step) ---
if (-not $summaryMd) {
    try {
        Invoke-RestMethod -Uri "$OllamaUrl/api/tags" -Method Get -TimeoutSec 10 | Out-Null
    } catch {
        Write-Host "[notice-summaries] no LLM API key and Ollama unreachable; announcement will use the raw changes text"
        exit 0
    }
    Write-Host "[notice-summaries] generating via local Ollama model=$OllamaModel"
    $ollamaBody = @{
        model    = $OllamaModel
        messages = @(
            @{ role = 'system'; content = $systemPrompt },
            @{ role = 'user'; content = $userPrompt }
        )
        stream   = $false
        # Qwen3+ puts reasoning into message.thinking and leaves content
        # empty unless thinking is disabled.
        think    = $false
        options  = @{ temperature = 0.3; num_predict = 700 }
    } | ConvertTo-Json -Depth 6
    try {
        $resp = Invoke-RestMethod -Uri "$OllamaUrl/api/chat" -Method Post -ContentType 'application/json; charset=utf-8' -Body $ollamaBody -TimeoutSec 300
        if ($resp.message -and $resp.message.content) {
            if (Test-SummaryMd -Text $resp.message.content) {
                $summaryMd = $resp.message.content.Trim()
                Write-Host "[notice-summaries] Ollama localized summaries generated"
            } else {
                Write-Host "::warning::Ollama response lacks valid ## zh / ## en / ## ru sections"
            }
        } else {
            Write-Host "::warning::Ollama returned empty content"
        }
    } catch {
        Write-Host "::warning::Ollama inference failed: $($_.Exception.Message)"
    }
}

if (-not $summaryMd) { exit 0 }

$outParent = Split-Path -Parent $OutputPath
if ($outParent -and -not (Test-Path $outParent)) {
    New-Item -ItemType Directory -Force -Path $outParent | Out-Null
}
[System.IO.File]::WriteAllText($OutputPath, $summaryMd + "`n", [System.Text.UTF8Encoding]::new($false))
Write-Host "[notice-summaries] summaries written to $OutputPath"
exit 0
