param(
    [string]$TestSetPath = ".\ml\test\test_set.json",
    [string]$ApiUrl = "http://localhost:11434/api/generate",
    [string]$Model = "mws-agent",
    [int]$TimeoutSec = 45,
    [switch]$SyntaxOnly   # <-- новый флаг: если указан, то не сверяем с эталоном
)

[Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8
$ErrorActionPreference = "Stop"

if (-not (Test-Path $TestSetPath)) {
    Write-Host "[FAIL] Test set not found: $TestSetPath" -ForegroundColor Red
    exit 1
}

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
        prompt = $case.prompt
        stream = $false
        options = @{
            num_predict = 256
            temperature = 0.2
        }
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri $ApiUrl -Method Post -Body $body -ContentType "application/json" -TimeoutSec $TimeoutSec
        $generated = $response.response.Trim()
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

    Write-Host "  Generated: $generated" -ForegroundColor Gray

    # Синтаксическая проверка
    $syntaxOk = $false
    try {
        $generated | docker exec -i ollama luac -p - 2>$null
        $syntaxOk = ($LASTEXITCODE -eq 0)
    } catch {
        Write-Host "  [WARN] Syntax check failed: $($_.Exception.Message)" -ForegroundColor Yellow
        $syntaxOk = $false
    }

    if ($syntaxOk) { $syntaxPass++ } else { $syntaxFail++ }

    if ($SyntaxOnly) {
        # В режиме "только синтаксис" считаем тест пройденным, если синтаксис корректен
        $match = $true
        $status = if ($syntaxOk) { "PASS" } else { "FAIL" }
        if ($syntaxOk) { $pass++ } else { $fail++ }
    } else {
        # Полный режим: нормализация и сравнение с эталоном
        $normalizedGen = ($generated -replace '\s+', ' ' -replace ';', '' -replace 'local', '').Trim().ToLower()
        $normalizedRef = ($case.reference -replace '\s+', ' ' -replace ';', '' -replace 'local', '').Trim().ToLower()
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