param([switch]$Rebuild)
$ErrorActionPreference = "Stop"
$Model = "mws-agent"
$Api = "http://localhost:11434/api/generate"

# 1. Rebuild (опционально)
if ($Rebuild) {
    Write-Host "[*] Rebuilding..." -ForegroundColor Cyan
    docker compose up -d --build --force-recreate ollama
    Start-Sleep -Seconds 45
}

# 2. API Health Check (гибкая проверка имени модели)
Write-Host "[*] Checking API..." -ForegroundColor Cyan
try {
    $h = Invoke-RestMethod -Uri "http://localhost:11434/api/tags" -TimeoutSec 10 -ErrorAction Stop

    # ← ГИБКАЯ ПРОВЕРКА: поддерживает "mws-agent", "mws-agent:latest", "mws-agent:v1"
    $found = $h.models.model | Where-Object { $_ -like "$Model*" -or $_ -eq $Model }

    if (-not $found) {
        Write-Host "[FAIL] Model '$Model' not found. Available: $($h.models.model -join ', ')" -ForegroundColor Red
        exit 1
    }
    Write-Host "[PASS] Model found: $($found -join ', ')" -ForegroundColor Green
}
catch {
    Write-Host "[FAIL] API Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 3. Test Cases (ASCII prompts only)
$tests = @(
  @{N="Add func"; P="Write Lua function add(a,b) return sum. Code only."; R="function.*add"},
  @{N="Table loop"; P="Lua: for loop over table. Code only."; R="for.*in.*pairs"},
  @{N="Error handle"; P="Lua: pcall example. Code only."; R="pcall"}
)

# 4. Run Tests
foreach ($t in $tests) {
  Write-Host "`n[TEST] $($t.N)" -ForegroundColor Cyan

  $body = @{
      model = $Model  # Ollama API авто-резолвит "mws-agent" → "mws-agent:latest"
      prompt = $t.P
      stream = $false
      options = @{
          num_ctx = 4096
          num_predict = 256
          temperature = 0.2
          batch = 1
          parallel = 1
      }
  } | ConvertTo-Json -Depth 10

  try {
    $r = Invoke-RestMethod -Uri $Api -Method Post -Body $body -ContentType "application/json" -TimeoutSec 60
    $out = $r.response.Trim()

    # Валидация вывода
    $hasMarkdown = $out -match '```'
    $startsOK = $out -match '^(function|local|return|for|if|do|while|end)'
    $matchesPattern = $out -match $t.R

    $ok = (-not $hasMarkdown) -and $startsOK -and $matchesPattern

    Write-Host "$($t.N): $(if($ok){"[PASS]"}else{"[FAIL]"})" -ForegroundColor $(if($ok){"Green"}else{"Red"})

    if (-not $ok) {
        $len = [Math]::Min(100, $out.Length)
        $preview = if ($len -gt 0) { $out.Substring(0, $len) } else { "(empty)" }
        Write-Host "  Output: $preview..." -ForegroundColor Yellow
        if ($hasMarkdown) { Write-Host "  [!] Contains markdown" -ForegroundColor Yellow }
        if (-not $startsOK) { Write-Host "  [!] Does not start with Lua keyword" -ForegroundColor Yellow }
        if (-not $matchesPattern) { Write-Host "  [!] Pattern mismatch" -ForegroundColor Yellow }
    }
  }
  catch {
      Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
  }
}
# ← foreach закрыт корректно