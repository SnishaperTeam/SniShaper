param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Channel,
    [Parameter(Mandatory = $false)][string]$PrereleaseSuffix = "",
    [Parameter(Mandatory = $false)][string]$PreviousTag = "",
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [Parameter(Mandatory = $false)][string]$OllamaUrl = "http://127.0.0.1:11434",
    [Parameter(Mandatory = $false)][string]$OllamaModel = "qwen2.5:0.5b",
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
    $rangeDesc = "对比自上个版本标签 $PreviousTag"
} else {
    $range = "HEAD"
    $rangeDesc = "首次发布，汇总全部提交记录"
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
$llmSource = $null

$batch = $commitList | Select-Object -First $LlmMaxCommits
$commitText = ($batch | ForEach-Object { "- $($_.Hash) $($_.Subject)" }) -join "`n"

$systemPrompt = @"
你是 SniShaper（Windows 本地代理工具）的开源项目发布说明撰写助手。
根据给出的 git commit 列表，用中文撰写简洁的变更摘要。
要求：
1. 按类别组织：新功能 / 问题修复 / 性能优化 / 重构 / 文档 / 构建与 CI / 其他
2. 每个类别用 Markdown 二级标题（###），类别下用简短要点总结（每点一句话，不要逐条罗列 commit）
3. 只总结实质变化，忽略依赖升级、格式化、merge 等噪音（依赖升级可合并为一条）
4. 不要输出 commit hash，不要输出多余的前言后语，直接输出摘要正文
5. 如果该版本是预发布版本（Beta/Alpha/RC），在开头用一句话说明这是预发布版本
"@

$userPrompt = "版本：$displayVersion`n以下是从上个发布标签到当前的所有提交（共 $totalCommits 条，展示前 $($batch.Count) 条）：`n$commitText"

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
    } | ConvertTo-Json -Depth 6
    try {
        $resp = Invoke-RestMethod -Uri "$OllamaUrl/api/chat" -Method Post -ContentType 'application/json; charset=utf-8' -Body $ollamaBody -TimeoutSec 300
        if ($resp.message -and $resp.message.content) {
            $llmSummary = $resp.message.content.Trim()
            $llmSource = "Ollama ($OllamaModel)"
            Write-Host "[release-notes] Ollama summary generated ($($llmSummary.Length) chars)"
        } else {
            Write-Host "::warning::Ollama returned empty response"
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
            $llmSource = "$LlmModel"
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
    [void]$sb.AppendLine("> **预发布版本**：该版本可能不稳定，部分功能会被移除或重做，不代表最终版本。")
    [void]$sb.AppendLine("")
} else {
    [void]$sb.AppendLine("# SniShaper $displayVersion")
    [void]$sb.AppendLine("")
}

[void]$sb.AppendLine("- 构建时间：$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')")
[void]$sb.AppendLine("- 版本来源：Package.appxmanifest")
[void]$sb.AppendLine("- 提交范围：$rangeDesc")
if ($llmSummary) {
    [void]$sb.AppendLine("- 变更摘要：AI 生成（$llmSource）")
}
[void]$sb.AppendLine("")

[void]$sb.AppendLine("## 变更摘要（共 $totalCommits 次提交）")
[void]$sb.AppendLine("")

if ($llmSummary) {
    [void]$sb.AppendLine($llmSummary)
    [void]$sb.AppendLine("")
} else {
    # Fallback: grouped commit list, cap each category to a few representative entries
    $sectionMap = @{
        feat     = '新功能'
        fix      = '问题修复'
        perf     = '性能优化'
        refactor = '重构'
        docs     = '文档'
        build    = '构建 / CI'
        test     = '测试'
        other    = '其他'
    }
    foreach ($key in @('feat', 'fix', 'perf', 'refactor', 'docs', 'build', 'test', 'other')) {
        $items = $groups[$key]
        if ($items.Count -eq 0) { continue }
        [void]$sb.AppendLine("### $($sectionMap[$key])（$($items.Count)）")
        [void]$sb.AppendLine("")
        $shown = $items | Select-Object -First 8
        foreach ($item in $shown) {
            [void]$sb.AppendLine("- $($item.Hash) $($item.Subject)")
        }
        if ($items.Count -gt 8) {
            [void]$sb.AppendLine("- ... 及其他 $($items.Count - 8) 条提交")
        }
        [void]$sb.AppendLine("")
    }
}

[void]$sb.AppendLine("## 贡献者（$($authors.Count) 人）")
[void]$sb.AppendLine("")
$sorted = $authors.GetEnumerator() | Sort-Object { $_.Value.Count } -Descending
foreach ($a in $sorted) {
    [void]$sb.AppendLine("- $($a.Value.Name) <$($a.Key)>：$($a.Value.Count) 次提交")
}
[void]$sb.AppendLine("")

[System.IO.File]::WriteAllText($OutputPath, $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Host "[release-notes] 已生成 $OutputPath"
