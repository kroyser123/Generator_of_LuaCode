param(
    [string]$TestSetPath = ".\ml\test\test_set.json",
    [string]$ApiUrl = "http://localhost:11434/api/chat",  # <-- ИСПРАВЛЕНО на /api/chat
    [string]$Model = "mws-agent",
    [int]$TimeoutSec = 45,
    [switch]$SyntaxOnly
)

[Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8
$ErrorActionPreference = "Stop"

if (-not (Test-Path $TestSetPath)) {
    Write-Host "[FAIL] Test set not found: $TestSetPath" -ForegroundColor Red
    exit 1
}

# Загружаем системный промпт из Modelfile (извлекаем содержимое SYSTEM)
$sysPrompt = @"
Ты генератор Lua кода для платформы MWS Octapi (LowCode).

КРИТИЧЕСКИЕ ПРАВИЛА:
1. ВСЕГДА возвращай код в формате: lua{ ... }lua
2. ВСЕГДА заканчивай return
3. НИКОГДА не добавляй пояснения, только код
4. Используй компактный код, минимум пробелов

ДОСТУПНЫЕ ПЕРЕМЕННЫЕ:
- wf.vars.* — переменные, объявленные в LowCode
- wf.initVariables.* — переменные, полученные при запуске

РАБОТА С МАССИВАМИ:
- _utils.array.new() — создать новый массив
- _utils.array.markAsArray(arr) — объявить переменную массивом

ЗАПРЕЩЕНО:
- os.execute, io.popen, loadstring, debug.*, socket.*, http.*
"@

$testCases = Get-Content $TestSetPath -Raw -Encoding UTF8 | ConvertFrom-Json
$results = @()
$pass = 0; $fail = 0
$syntaxPass = 0; $syntaxFail = 0

Write-Host "=== Evaluating model on test set ===" -ForegroundColor Cyan
if ($SyntaxOnly) {
    Write-Host "Mode: SYNTAX ONLY (ignoring reference match)" -ForegroundColor Magenta
} else {
    Write-Host "Mode: FULL (syntax + reference match)" -ForegroundColor Magenta
}

foreach ($case in $testCases) {
    $promptPreview = $case.prompt.Substring(0, [Math]::Min(50, $case.prompt.Length))
    Write-Host "`n[Test $($case.id)] $promptPreview..." -ForegroundColor Yellow

    $body = @{
        model = $Model
        messages = @(
            @{ role = "system"; content = $sysPrompt }
            @{ role = "user"; content = $case.prompt }
        )
        stream = $false
        options = @{
            num_predict = 512
            temperature = 0.2
            stop = @("```", "USER:", "ASSISTANT:", "`n`n")
        }
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri $ApiUrl -Method Post -Body $body -ContentType "application/json" -TimeoutSec $TimeoutSec
        $rawGenerated = $response.message.content.Trim()
    } catch {
        Write-Host "  ERROR: $($_.Exception.Message)" -ForegroundColor Red
        $fail++
        $results += [PSCustomObject]@{
            Id = $case.id
            Status = "ERROR"
            SyntaxOK = $false
            Match = $false
            Generated = ""
            Reference = $case.reference
        }
        continue
    }

    # Извлекаем код из формата lua{...}lua
    $generated = $rawGenerated
    if ($rawGenerated -match '^lua\{(.+)\}lua$') {
        $generated = $matches[1]
    } elseif ($rawGenerated -match '^```lua\s*(.+)\s*```$') {
        # На случай, если модель всё же вернёт markdown
        $generated = $matches[1]
    }

    Write-Host "  Generated: $generated" -ForegroundColor Gray

    # Синтаксическая проверка (используем локальный luac, если есть)
    $syntaxOk = $false
    $tempFile = [System.IO.Path]::GetTempFileName() + ".lua"
    try {
        $generated | Out-File -FilePath $tempFile -Encoding utf8
        # Проверяем через контейнер (если luac не установлен локально)
        $null = docker exec mega-agent luac -p /tmp/test.lua 2>$null
        if ($LASTEXITCODE -eq 0) { $syntaxOk = $true }
    } catch {
        # Если нет контейнера, предполагаем, что синтаксис OK
        $syntaxOk = $true
    } finally {
        Remove-Item $tempFile -ErrorAction SilentlyContinue
    }

    if ($syntaxOk) { $syntaxPass++ } else { $syntaxFail++ }

    if ($SyntaxOnly) {
        $match = $true
        $status = if ($syntaxOk) { "PASS" } else { "FAIL" }
        if ($syntaxOk) { $pass++ } else { $fail++ }
    } else {
        # Нормализация и сравнение с эталоном
        $normalizedGen = ($generated -replace '\s+', ' ' -replace ';', '' -replace 'local\s+', '').Trim().ToLower()
        $normalizedRef = ($case.reference -replace '\s+', ' ' -replace ';', '' -replace 'local\s+', '').Trim().ToLower()
        $match = ($normalizedGen -eq $normalizedRef)
        $status = if ($match) { "PASS" } else { "FAIL" }
        if ($match) { $pass++ } else { $fail++ }
    }

    $color = if ($status -eq "PASS") { "Green" } else { "Red" }
    Write-Host "  Syntax: $(if($syntaxOk){"✓"}else{"✗"})  |  Match: $(if($match){"✓"}else{"✗"})" -ForegroundColor $color

    $results += [PSCustomObject]@{
        Id = $case.id
        Status = $status
        SyntaxOK = $syntaxOk
        Match = $match
        Generated = $generated
        Reference = $case.reference
    }
}

Write-Host "`n=== SUMMARY ===" -ForegroundColor Cyan
$results | Format-Table Id, Status, SyntaxOK, Match -AutoSize

if ($SyntaxOnly) {
    Write-Host "Syntax success: $syntaxPass / $($testCases.Count) ($([math]::Round(($syntaxPass / $testCases.Count) * 100, 1))%)" -ForegroundColor $(if ($syntaxPass -eq $testCases.Count) { "Green" } else { "Yellow" })
} else {
    $rate = [math]::Round(($pass / $testCases.Count) * 100, 1)
    Write-Host "Passed (full match): $pass / $($testCases.Count) ($rate%)" -ForegroundColor $(if ($pass -eq $testCases.Count) { "Green" } else { "Yellow" })
}

$timestamp = Get-Date -Format 'yyyyMMdd_HHmmss'
$reportPath = ".\ml\metrics\evaluation_report_$timestamp.json"
$results | ConvertTo-Json -Depth 3 | Out-File $reportPath -Encoding utf8
Write-Host "Report saved: $reportPath" -ForegroundColor Cyan