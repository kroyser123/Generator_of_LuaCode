param()
$ErrorActionPreference = "Stop"
$Model = "mws-agent"
$Api = "http://localhost:11434/api/chat"
$SysFile = Join-Path $PSScriptRoot "..\prompts\system.txt"
if(-not (Test-Path $SysFile)) { Write-Host "[FAIL] system.txt not found" -ForegroundColor Red; exit 1 }
$System = Get-Content $SysFile -Raw -Encoding UTF8
# Динамическая сборка backticks без литералов
$bt = [string]([char]96) * 3
$stop = @($bt, "USER:", "ASSISTANT:", "`n`n")
$body = @{
    model = $Model
    messages = @(
        @{ role = "system"; content = $System }
        @{ role = "user"; content = "Write a Lua function to calculate factorial of n." }
    )
    stream = $false
    options = @{
        num_ctx = 4096
        num_predict = 256
        temperature = 0.2
        stop = $stop
    }
} | ConvertTo-Json -Depth 10
try {
    $r = Invoke-RestMethod -Uri $Api -Method Post -Body $body -ContentType "application/json" -TimeoutSec 45
    $out = $r.message.content.Trim()
    Write-Host "=== OUTPUT ===" -ForegroundColor Cyan
    Write-Host $out -ForegroundColor White
    # Валидация
    $hasForbidden = $out -match "os\.|io\.popen|loadstring|debug\.|socket\.|http\."
    $hasMarkdown = $out -match $bt
    $hasPrefix = $out -match "USER:|ASSISTANT:"
    $hasLua = $out -match "^(?i)(function|local|return)"
    $ok = -not $hasForbidden -and -not $hasMarkdown -and -not $hasPrefix -and $hasLua
    Write-Host "`n=== VALIDATION ===" -ForegroundColor Cyan
    Write-Host "No forbidden ops: $(if(-not $hasForbidden){"✓"}else{"✗"})" -ForegroundColor $(if(-not $hasForbidden){"Green"}else{"Red"})
    Write-Host "No markdown:      $(if(-not $hasMarkdown){"✓"}else{"✗"})" -ForegroundColor $(if(-not $hasMarkdown){"Green"}else{"Red"})
    Write-Host "No prefixes:      $(if(-not $hasPrefix){"✓"}else{"✗"})" -ForegroundColor $(if(-not $hasPrefix){"Green"}else{"Red"})
    Write-Host "Starts with Lua:  $(if($hasLua){"✓"}else{"✗"})" -ForegroundColor $(if($hasLua){"Green"}else{"Red"})
    Write-Host "`nCompliance: $(if($ok){"✓ PASS"}else{"✗ FAIL"})" -ForegroundColor $(if($ok){"Green"}else{"Red"})
} catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}