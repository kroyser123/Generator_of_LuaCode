# Prompt Compliance Report
Generated: 2026-04-11 18:50:33 | Model: mws-agent:latest | Compliance: ISSUES DETECTED
## Results Summary
| Prompt File | Status | Details |
|-------------|--------|---------|
| system.txt | FAIL | Does not start with valid Lua construct |
| cot.txt | FAIL | Missing Lua output structure |
| correction.txt | FAIL | Missing corrected function skeleton |
| clarify.txt | FAIL | Missing clarification question |
| optimization.txt | FAIL | Missing Lua output structure |
| security.txt | FAIL | Missing safe fallback |
| validation.txt | FAIL | Missing aggregation structure |
| few_shot.json | PASS | Structure valid |

## Metrics
- Total: 8 | Passed: 1 | Failed: 7
- Compliance Rate: 12.5%
## Recommendations
1. Adjust prompt framing for contextual files.
2. Verify stop tokens in Modelfile.
3. Re-run.
