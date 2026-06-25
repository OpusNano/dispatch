# Release Checklist

## Build

```bash
go build -o dispatch ./cmd/dispatch
```

## Test

```bash
go test -count=1 -timeout=60s ./...
```

## Docker Build

```bash
docker build -t dispatch:rc .
```

## Run

### Docker
```bash
mkdir -p ./config
docker run -d --name dispatch \
  -p 18087:18087 \
  -e OPENROUTER_API_KEY=sk-or-... \
  -v ./config:/config \
  dispatch:rc
```

(Optional non-root: add `--user 65532:65532` and `chown 65532:65532 ./config`.)

### Docker Compose
```bash
# Edit .env with your OpenRouter API key
docker compose up -d --build
```

### Plain Binary
```bash
OPENROUTER_API_KEY=sk-or-... ./dispatch --config /path/to/router.yaml
```

## OpenCode Settings

| Setting  | Value                       |
|----------|-----------------------------|
| Base URL | http://localhost:18087/v1   |
| API Key  | placeholder (server-side)   |
| Model    | dispatch/auto                |

## Verification

```bash
# Smoke test (no API key needed)
./scripts/smoke.sh http://localhost:18087

# Health check
curl http://localhost:18087/health

# Debug classification
curl -s -X POST http://localhost:18087/debug/route \
  -H "Content-Type: application/json" \
  -d '{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}]}'

# Stats
curl -s http://localhost:18087/debug/stats | jq
```

## Common Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `permission denied` on first run | `/config` not owned by UID 65532 | `chown 65532:65532 ./config` |
| `OPENROUTER_API_KEY environment variable not set` | Missing API key | Set `-e OPENROUTER_API_KEY=...` or use `.env` |
| Upstream 401 | Invalid/expired API key | Check key at openrouter.ai/keys |
| Upstream 429 | Rate limited | Wait or upgrade plan |
| "Provider returned error" | Provider rate-limited/down, fallbacks disabled | Enable `allow_fallbacks: true` or empty `provider.order: []` |
| 502/503 from OpenRouter | Provider unavailable | Let OpenRouter fallback; check `/debug/stats` for patterns |
| Port 18087 in use | Conflicting service | Change `server.listen` and port mapping |
| OpenCode shows only `dispatch/auto` | Expected behavior | Check `X-Dispatch-Model` header for actual model |
| OpenRouter shows App = Unknown | Empty http_referer/site_title | Set both in config; see README troubleshooting |
