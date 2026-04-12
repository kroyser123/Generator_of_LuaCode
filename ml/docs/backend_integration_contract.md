> Backend Integration Contract: ML → Backend

## Model Endpoint
```yaml
service: ollama
endpoint: http://ollama:11434/api/chat  # Preferred over /api/generate
method: POST