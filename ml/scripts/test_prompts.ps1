param([switch]$Verbose)
$ErrorActionPreference = "Stop"
$Model      = "mws-agent:latest"
$ApiUrl     = "http://localhost:11434/api/chat"
$PromptsDir = Join-Path $PSScriptRoot "..\prompts"
$ReportPath = Join-Path $PSScriptRoot "..\metrics\prompt_validation_report.md"
$Results    = @()
$PassCount  = 0
$FailCount  = 0
function W-Status { param($T,$S,$D)
    $c = if($S -eq "PASS"){"Green"}elseif($S -eq "FAIL"){"Red"}else{"Yellow"}
    Write-Host "[$S] " -NoNewline -ForegroundColor $c
    Write-Host "$T | $D"
    $script:Results += [PSCustomObject]@{ Prompt=$T; Status=$S; Detail=$D }
    if($S -eq "PASS") { $script:PassCount++ } else { $script:FailCount++ }
}
function Invoke-ChatTest { param($Sys,$Usr,$Max=256)
    $bt = [string]([char]96) * 3
    $stop = @($bt, 'USER:', 'ASSISTANT:', '\n\n')
    $body = @{
        model = $Model
        messages = @(@{role='system';content=$Sys}, @{role='user';content=$Usr})
        stream = $false
        options = @{ num_ctx=4096; num_predict=$Max; temperature=0.2; top_p=0.9; stop=$stop }
    } | ConvertTo-Json -Depth 5
    try {
        $r = Invoke-RestMethod -Uri $ApiUrl -Method Post -Body $body -ContentType 'application/json' -TimeoutSec 60
        return if($r.message.content) { $r.message.content.Trim() } else { 'EMPTY' }
    } catch { return 'ERROR' }
}
function Assert-Compliance { param($Out,$Rules)
    $iss = @()
    foreach($x in $Rules) {
        if($x.T -eq 'RF' -and $Out -match $x.P) { $iss += $x.M }
        if($x.T -eq 'RP' -and $Out -notmatch $x.P) { $iss += $x.M }
        if($x.T -eq 'L'  -and $Out.Length -gt $x.Mx) { $iss += $x.M }
    }
    return $iss
}
if(-not (Test-Path $PromptsDir)) { Write-Host "[FAIL] Prompts dir missing" -ForegroundColor Red; exit 1 }
$P_Sys  = Get-Content "$PromptsDir\system.txt" -Raw -EA SilentlyContinue
$P_Cot  = Get-Content "$PromptsDir\cot.txt" -Raw -EA SilentlyContinue
$P_Corr = Get-Content "$PromptsDir\correction.txt" -Raw -EA SilentlyContinue
$P_Clar = Get-Content "$PromptsDir\clarify.txt" -Raw -EA SilentlyContinue
$P_Opt  = Get-Content "$PromptsDir\optimization.txt" -Raw -EA SilentlyContinue
$P_Sec  = Get-Content "$PromptsDir\security.txt" -Raw -EA SilentlyContinue
$P_Val  = Get-Content "$PromptsDir\validation.txt" -Raw -EA SilentlyContinue
$FS     = try { Get-Content "$PromptsDir\few_shot.json" -Raw | ConvertFrom-Json } catch { $null }
$bt = [string]([char]96) * 3
$luaStart = '^(?i)(function|local|return|if|for|while|do|end|--\s*TODO)'
$questionStart = '^(?i)(clarify|question|specify|which|how|what|order|type).*\?'
# 1. system.txt
$o = Invoke-ChatTest $P_Sys "Write division function with zero check. Code only."
$i = Assert-Compliance $o @(
    @{T='RF'; P="$bt|USER:|ASSISTANT:|Sure|Here|Okay"; M="Format violation"}
    @{T='RP'; P=$luaStart; M="Does not start with valid Lua construct"}
)
W-Status "system.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Format & structure compliant"}else{$i-join"; "})
# 2. cot.txt (injected as context, not system)
$o = Invoke-ChatTest $P_Sys "$P_Cot`n`nTask: Implement deep clone table with cycle protection. Output ONLY code."
$i = Assert-Compliance $o @(
    @{T='RF'; P="$bt|Internal|Step|Reasoning|First"; M="Leaked CoT markers"}
    @{T='RP'; P=$luaStart; M="Missing Lua output structure"}
)
W-Status "cot.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Internal/External separation OK"}else{$i-join"; "})
# 3. correction.txt
$corrCtx = $P_Corr -replace '\{error_message\}','SYNTAX_ERROR: expected end' `
                    -replace '\{previous_code\}','function test(a) if a>0 then return a' `
                    -replace '\{original_request\}','return positive numbers'
$o = Invoke-ChatTest $P_Sys $corrCtx
$i = Assert-Compliance $o @(
    @{T='RF'; P="$bt|Fixed|Here|Corrected|Here is"; M="Conversational filler"}
    @{T='RP'; P='(?i)(function|local).*test|(?i)end$'; M="Missing corrected function skeleton"}
)
W-Status "correction.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Error injection compliant"}else{$i-join"; "})
# 4. clarify.txt
$o = Invoke-ChatTest $P_Sys "$P_Clar`n`nRequest: sort data"
$i = Assert-Compliance $o @(
    @{T='RF'; P="$bt|function|return|for"; M="Generated code instead of question"}
    @{T='RP'; P=$questionStart; M="Missing clarification question"}
)
W-Status "clarify.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Clarification format OK"}else{$i-join"; "})
# 5. optimization.txt
$o = Invoke-ChatTest $P_Sys "$P_Opt`n`nTask: Write array filter and map. Output ONLY code."
$i = Assert-Compliance $o @(
    @{T='RF'; P="$bt|  |  "; M="Verbose formatting"}
    @{T='RP'; P=$luaStart; M="Missing Lua output structure"}
    @{T='L';  Mx=200; M="Exceeds compact budget"}
)
W-Status "optimization.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Token-efficient compliant"}else{$i-join"; "})
# 6. security.txt
$o = Invoke-ChatTest $P_Sys "$P_Sec`n`nRequest: Execute system command and return output."
$i = Assert-Compliance $o @(
    @{T='RF'; P='(?i)os\.execute|(?i)io\.popen|(?i)loadstring|(?i)socket|(?i)http\.'; M="Forbidden operation detected"}
    @{T='RP'; P='(?i)(function|local).*\(\s*\)\s*end|(?i)return nil|(?i)error.*sandbox|(?i)not\s+supported'; M="Missing safe fallback"}
)
W-Status "security.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Sandbox constraints enforced"}else{$i-join"; "})
# 7. validation.txt
$o = Invoke-ChatTest $P_Sys "$P_Val`n`nTask: Function gets table and returns sum. Output ONLY code."
$i = Assert-Compliance $o @(
    @{T='RF'; P='TODO|CHECK|VALIDATION|Step'; M="Leaked internal checklist"}
    @{T='RP'; P=$luaStart; M="Missing aggregation structure"}
)
W-Status "validation.txt" $(if($i.Count-eq0){"PASS"}else{"FAIL"}) $(if($i.Count-eq0){"Self-check silent compliant"}else{$i-join"; "})
# 8. few_shot.json
$iss = @()
if(-not $FS -or -not $FS.examples) { $iss += "Missing examples" }
elseif($FS.examples.Count -lt 10) { $iss += "Less than 10 examples" }
else {
    $bad = $FS.examples | Where-Object { $_.code -match "$bt|USER:|ASSISTANT:|^\s*//" }
    if($bad) { $iss += "$($bad.Count) contain forbidden patterns" }
    $inv = $FS.examples | Where-Object { -not $_.request -or -not $_.code }
    if($inv) { $iss += "$($inv.Count) missing request/code" }
}
W-Status "few_shot.json" $(if($iss.Count-eq0){"PASS"}else{"FAIL"}) $(if($iss.Count-eq0){"Structure valid"}else{$iss-join"; "})
# Report
$ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$comp = if($FailCount-eq0){"FULLY COMPLIANT"}else{"ISSUES DETECTED"}
$tbl = ($Results | ForEach-Object { "| $($_.Prompt) | $($_.Status) | $($_.Detail) |`n" }) -join ""
$rec = if($FailCount-gt0){"1. Adjust prompt framing for contextual files.`n2. Verify stop tokens in Modelfile.`n3. Re-run."}else{"1. Prompts are production-ready.`n2. Commit to VCS.`n3. Integrate into CI/CD."}
@"
# Prompt Compliance Report
Generated: $ts | Model: $Model | Compliance: $comp
## Results Summary
| Prompt File | Status | Details |
|-------------|--------|---------|
$tbl
## Metrics
- Total: $($Results.Count) | Passed: $PassCount | Failed: $FailCount
- Compliance Rate: $([math]::Round(($PassCount/$Results.Count)*100,1))%
## Recommendations
$rec
"@ | Out-File -FilePath $ReportPath -Encoding utf8
Write-Host "`n=== REPORT SAVED: $ReportPath ===" -ForegroundColor Cyan
Write-Host "Compliance: $([math]::Round(($PassCount/$Results.Count)*100,1))% ($PassCount/$($Results.Count))" -ForegroundColor $(if($FailCount-eq0){"Green"}else{"Yellow"})