# Dispatch

Bespoke OpenRouter-only complexity router for OpenCode.

Classifies chat completion requests into four levels (easy, medium, hard, critical) and routes to configured OpenRouter models. Uses evidence-based complexity routing with active task frame extraction to prevent context contamination.

## Quick Start

```bash
# Create config dir with correct ownership (container runs as UID 65532)
mkdir -p ./config
chown 65532:65532 ./config

# Edit .env with your OpenRouter API key
# or set OPENROUTER_API_KEY environment variable

# Build and run
docker compose up -d --build

# Or build manually
docker build -t dispatch .
docker run -d \
  --name dispatch \
  -p 18087:18087 \
  --env-file .env \
  -v ./config:/config \
  dispatch
```

The router automatically generates `/config/router.yaml`, `/config/DISPATCH.md`, and `/config/exemplars.yaml` on first run. Config changes are auto-reloaded without restart.

**Warnings:**
- The router listens on **plain HTTP** (`:18087`). For production, place it behind a TLS-terminating reverse proxy.
- The `/config` directory **must be writable** by UID 65532 on first run. After generation, files need only read access.
- Never put your API key in the config file. Always use the `OPENROUTER_API_KEY` environment variable via `.env`.
- Prompt text is **not logged** by default (`log_prompts: false`). Only enable it if you understand the privacy impact.
- Trace mode (`trace_requests: false`) is metadata-only — it never logs prompt content.

## OpenCode Setup

Configure OpenCode to use Dispatch:

| Setting    | Value                           |
|------------|---------------------------------|
| Base URL   | `http://localhost:18087/v1`     |
| API Key    | `placeholder` (key is server-side) |
| Model      | `dispatch/auto`                  |

OpenCode may display only `dispatch/auto` — the final model selection happens inside the router and is **not** part of the model list. The selected model is visible via:
- Response header `X-Dispatch-Model`
- Structured log output (stderr)
- `GET /debug/stats` endpoint
- `GET /debug/request?id=<request_id>` metadata lookup

### Optional Session/Task Headers

If your client can send `X-Dispatch-Session-Id` and `X-Dispatch-Task-Id` headers, Dispatch will use them for task-scoped session escalation. If not, Dispatch derives task keys from the active task frame. **OpenCode does not need to send custom headers — routing works without them.**

## Active Task Frame Routing

Dispatch does **not** classify the full conversation. Each request is classified using only the **active task frame**:

- **New standalone user turn** → frame starts at the latest user message. Old hard debugging context, old stack traces, old tool outputs are excluded.
- **Continuation detected** (e.g., "same error still happens", "try again", "tests still failing") → frame extends back to the original task boundary.
- **No new user turn after tool results** → frame includes the last user instruction and all tool results after it.

This prevents a long hard debugging session from contaminating an unrelated easy question in the same chat.

### Manual Overrides

- Model alias: `dispatch/easy`, `dispatch/medium`, `dispatch/hard`, `dispatch/critical`, `dispatch/auto`
- Header: `X-Dispatch-Level: easy|medium|hard|critical`
- Precedence: **Header** > Model alias > Auto classification

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Route and forward to OpenRouter |
| `/debug/route` | POST | Classify only (no upstream call), returns level/scores/reasons/frame |
| `/debug/stats` | GET | In-memory stats: request counts, by-level, by-model, by-status, avg duration, uptime, reload count |
| `/debug/request?id=<request_id>` | GET | Metadata lookup for a past request (no prompt text, bounded ring buffer) |
| `/debug/feedback` | POST | Submit feedback (disabled by default) |
| `/health` | GET | Health check |
| `/version` | GET | Build info |

### Debug Classification

```bash
curl -s -X POST http://localhost:18087/debug/route \
  -H "Content-Type: application/json" \
  -d '{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function to sort an array"}]}' | jq
```

Response includes `level`, `model`, `scores`, `reasons`, `analysis`, `frame`, `request_id`.

### Stats

```bash
curl -s http://localhost:18087/debug/stats | jq
```

### Request Metadata Lookup

```bash
curl -s "http://localhost:18087/debug/request?id=<request_id>" | jq
```

Returns metadata only — no prompt text, no tool output.

### Feedback (disabled by default)

Enable in config:
```yaml
debug:
  feedback_enabled: true
  feedback_path: "/config/feedback.jsonl"
```

Then:
```bash
curl -s -X POST http://localhost:18087/debug/feedback \
  -H "Content-Type: application/json" \
  -d '{"request_id":"abc123","expected_level":"hard","note":"should have been hard"}'
```

## Automatic Config Reload

Edit `/config/router.yaml` and the router picks up changes automatically (default: every 3 seconds). No restart needed.

- If reload succeeds, new requests use the new config. In-flight requests continue with the old config snapshot.
- If reload fails (bad YAML, invalid config), the old config stays active and the error is logged.
- API key is **not** reloaded from config — it stays env-driven.
- `--check-config` remains available for manual validation.
- Reload stats are visible at `/debug/stats` (`config_reload_count`, `last_config_reload_unix`).

## Architecture

```
OpenCode → Dispatch (classify active task frame) → OpenRouter
                 ↓
            POST /v1/chat/completions
            POST /debug/route
            GET  /debug/stats
            GET  /debug/request?id=...
            POST /debug/feedback
            GET  /health
            GET  /version
```

## Levels & Models

| Level    | Default Model               |
|----------|-----------------------------|
| easy     | deepseek/deepseek-v4-flash  |
| medium   | deepseek/deepseek-v4-flash  |
| hard     | deepseek/deepseek-v4-pro    |
| critical | z-ai/glm-5.2                |

## Configuration

Edit `/config/router.yaml`. See `/config/DISPATCH.md` for full documentation.

```yaml
openrouter:
  base_url: "https://openrouter.ai/api/v1"
  api_key_env: "OPENROUTER_API_KEY"

server:
  listen: ":18087"
  max_body_size: 26214400  # 25 MiB

levels:
  easy:
    model: "deepseek/deepseek-v4-flash"
  medium:
    model: "deepseek/deepseek-v4-flash"
  hard:
    model: "deepseek/deepseek-v4-pro"
  critical:
    model: "z-ai/glm-5.2"

debug:
  log_decisions: true
  log_prompts: false
  trace_requests: false
  request_index_enabled: true
  request_index_size: 500
  feedback_enabled: false

config_reload:
  enabled: true
  poll_interval_seconds: 3
```

## Development

```bash
# Tests
go test ./...

# Lint
go vet ./...

# Build
go build -o dispatch ./cmd/dispatch

# Check config
./dispatch --check-config --config /path/to/router.yaml

# Run
OPENROUTER_API_KEY=sk-or-... ./dispatch --config /path/to/router.yaml
```

## Response Headers

| Header | Description |
|--------|-------------|
| `X-Dispatch-Request-Id` | Unique request ID for log correlation |
| `X-Dispatch-Level` | Selected level |
| `X-Dispatch-Model` | Routed model ID |
| `X-Dispatch-Score-Total` | Composite score |
| `X-Dispatch-Score-Complexity` | Complexity sub-score |
| `X-Dispatch-Score-Risk` | Risk sub-score |
| `X-Dispatch-Score-Agent-Pressure` | Agent pressure sub-score |
| `X-Dispatch-Reasons` | Classification reasons (truncated) |

## Security

- API key from env var only, never in config files.
- Prompts not logged by default. Do not enable `log_prompts` unless you accept the privacy risk.
- Trace mode is metadata-only — logs message structure, not content.
- Request metadata index stores hashes and metadata, never prompt text.
- Request body size capped.
- Streaming responses not buffered.
- Client headers not forwarded upstream.
- Request ID in response header and logs for correlation.
- **Messages never modified**: The upstream request forwarded to OpenRouter preserves the original `messages`, `tools`, `response_format`, and all unknown fields. Dispatch only changes `model` and configured `provider` fields. Debug information is never injected into message content.

## Content Preservation Guarantee

Dispatch classifies requests using the active task frame for **routing decisions only**. The upstream request forwarded to OpenRouter is a byte-preserving modification of the original:

| Field | Modified? | Details |
|-------|:---------:|---------|
| `model` | Yes | Replaced with the selected level's model ID |
| `provider` | Yes | Merged with level config (order, only, ignore, data_collection, allow_fallbacks) |
| `messages` | **Never** | All roles, content, tool_calls, tool_call_id, content arrays preserved |
| `tools` | **Never** | All tool definitions preserved |
| `tool_choice` | **Never** | Preserved as-is |
| `response_format` | **Never** | Preserved as-is |
| `stream` | **Never** | Preserved as-is |
| `temperature`, `max_tokens`, `top_p`, etc | **Never** | All unknown fields preserved |
| `X-Dispatch-*` headers, `request_id`, debug text | **Never injected** | Debug info stays in response headers, logs, and `/debug` endpoints |

## Smoke Test

```bash
# Start Dispatch locally, then:
./scripts/smoke.sh
# Or against a different host:
./scripts/smoke.sh http://localhost:18087
```

## Troubleshooting

### Port already in use
Change `server.listen` in router.yaml, update the Docker port mapping (`-p`), and restart.

### Config dir permission denied
```
dispatch: cannot write /config/router.yaml: permission denied
```
The container runs as UID 65532. Fix with:
```bash
chown 65532:65532 ./config
```

### Missing API key
```
dispatch: OPENROUTER_API_KEY environment variable not set
```
Set it via `.env` file or `-e OPENROUTER_API_KEY=sk-or-...`.

### Upstream 401
The OpenRouter API key is invalid, expired, or has no credits. Check your key at https://openrouter.ai/keys.

### Upstream 429
OpenRouter rate limiting. Reduce request volume or upgrade your plan.

### OpenCode only shows dispatch/auto
This is expected. The router selects the actual model internally per request. Check the `X-Dispatch-Model` response header or container logs to see which model was used.

### Config reload failed
If you save a bad config, the router keeps the old config active and logs the error. Check stderr logs for "config reload: validation failed".
