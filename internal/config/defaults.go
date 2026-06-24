package config

const defaultConfigYAML = `# Dispatch — OpenRouter complexity router for OpenCode.
# See DISPATCH.md for full documentation and safe editing guidance.
#
# API keys are loaded from environment variables, not from this file.
# Set OPENROUTER_API_KEY in your .env file (copy .env.example → .env).

# ─────────────────────────────────────────────────────────────
#  OpenRouter  — upstream connection
# ─────────────────────────────────────────────────────────────

openrouter:
  base_url: "https://openrouter.ai/api/v1"
  api_key_env: "OPENROUTER_API_KEY"
  validate_models_on_start: false
  http_referer: "https://github.com/OpusNano/dispatch"
  site_title: "Dispatch"

# ─────────────────────────────────────────────────────────────
#  Server  — HTTP listen settings
# ─────────────────────────────────────────────────────────────

server:
  listen: ":18087"
  max_body_size: 26214400
  read_timeout_seconds: 30
  write_timeout_seconds: 0

# ─────────────────────────────────────────────────────────────
#  Model Profiles
# ─────────────────────────────────────────────────────────────
#
#  Define reusable model profiles here.  The YAML key is the
#  profile name (your choice).  'id' is the OpenRouter model ID.
#  Levels below reference profiles with 'use:'.
#
#  Empty provider.order: [] means "let OpenRouter choose."

model_profiles:
  deepseek_flash:
    id: "deepseek/deepseek-v4-flash"
    provider:
      order: []
      allow_fallbacks: true
      data_collection: "deny"

  deepseek_pro:
    id: "deepseek/deepseek-v4-pro"
    provider:
      order: []
      allow_fallbacks: true
      data_collection: "deny"

  glm_52:
    id: "z-ai/glm-5.2"
    provider:
      order: []
      allow_fallbacks: true
      data_collection: "deny"

# ─────────────────────────────────────────────────────────────
#  Routing Levels
# ─────────────────────────────────────────────────────────────
#
#  Dispatch classifies each request and resolves the level to
#  a model profile.  Use 'use:' to reference a profile above,
#  or 'model:' for a direct inline override.

levels:
  easy:
    use: deepseek_flash

  medium:
    use: deepseek_flash

  hard:
    use: deepseek_pro

  critical:
    use: glm_52

# ─────────────────────────────────────────────────────────────
#  Thresholds  — score → level mapping
# ─────────────────────────────────────────────────────────────

thresholds:
  easy: 0
  easy_max: 20
  medium_max: 45
  hard_max: 70
  risk_critical_override: 50
  risk_hard_floor: 35
  agent_pressure_critical_override: 50

# ─────────────────────────────────────────────────────────────
#  Scoring  — dimension caps and weights
# ─────────────────────────────────────────────────────────────

scoring:
  complexity_cap: 40
  risk_cap: 60
  agent_pressure_cap: 30
  downgrade_cap: 30
  weights:
    complexity: 1.0
    risk: 1.0
    agent_pressure: 0.8
    downgrade: -1.0

# ─────────────────────────────────────────────────────────────
#  Patterns  — evidence-detection rules
# ─────────────────────────────────────────────────────────────

patterns:
  - id: coding_intent
    regex: 'write|implement|create|build|generate|code|program|script|function|class|module|handler|endpoint|api|service|controller|refactor|debug|fix|optimize|redesign|restructure|migrate|architecture'
    dimension: complexity
    weight: 12
    reason: "coding intent"

  - id: tool_call_present
    match_tools: true
    dimension: complexity
    weight: 8
    reason: "tool use detected"

  - id: response_format_json
    match_response_format: true
    dimension: complexity
    weight: 10
    reason: "structured output required"

  - id: code_block
    regex: "\\x60[\\s\\S]*?\\x60"
    dimension: complexity
    weight: 8
    reason: "code block present"

  - id: diff_patch
    regex: '(?m)^@@|^\+\+\+ |^--- |^diff --git'
    dimension: complexity
    weight: 10
    reason: "diff/patch present"

  - id: stack_trace
    regex: '(?m)\s+at .+\(.+:\d+\)|Traceback|panic:|goroutine \d+'
    dimension: agent_pressure
    weight: 18
    reason: "stack trace"

  - id: failed_tests
    regex: 'FAIL(URE)?|tests? (failed|passing)|\d+ failing'
    dimension: agent_pressure
    weight: 18
    reason: "failing tests"

  - id: compile_error
    regex: 'compile error|syntax error|cannot find symbol|undefined (reference|symbol)|build failed'
    dimension: agent_pressure
    weight: 15
    reason: "compile/build error"

  - id: tool_error
    regex: 'tool (call )?(failed|error)|exit code [1-9]|command not found'
    dimension: agent_pressure
    weight: 15
    reason: "tool error"

  - id: repeated_failure
    regex: 'still failing|same error|again.*error|error persists|not working still|keeps failing|repeatedly'
    dimension: agent_pressure
    weight: 20
    reason: "repeated failure phrase"

  - id: multiline_code_count
    regex: '(?m)^\s*(func |def |class |public |private |import |from )'
    dimension: complexity
    weight: 3
    per_match: true
    cap: 12
    reason: "multi-line code structure"

  - id: multi_file_refactor
    regex: 'refactor|across (several|multiple) files|multiple files|architecture|redesign|restructure'
    dimension: complexity
    weight: 15
    reason: "multi-file/architecture intent"

  - id: multi_step_reasoning
    regex: 'step[- ]by[- ]step|first .+ then .+ finally|plan:|multi-step|chain of thought'
    dimension: complexity
    weight: 10
    reason: "multi-step reasoning"

  - id: production_topic
    regex: 'production|prod environment|live (system|traffic)|customer[- ]facing'
    dimension: risk
    weight: 5
    reason: "production context (metadata)"

  - id: security_topic
    regex: 'security (vuln|flaw)|CVE|authentication|authorization|OAuth|privilege escalat|XSS|SQL injection|secret leak'
    dimension: risk
    weight: 5
    reason: "security/auth context (metadata)"

  - id: payment_topic
    regex: 'payment|billing|charge|refund|stripe|credit card|transaction'
    dimension: risk
    weight: 5
    per_match: true
    cap: 10
    reason: "payment/billing context (metadata)"

  - id: database_topic
    regex: 'migration|rollback|schema change|ALTER TABLE|migrate'
    dimension: risk
    weight: 5
    reason: "database context (metadata)"

  - id: secrets_topic
    regex: 'secret|API key|private key|credential|token leak|\.env'
    dimension: risk
    weight: 5
    reason: "secrets/credentials context (metadata)"

  - id: destructive_action_evidence
    regex: '(?i)\b(drop table|drop column|truncate|delete all|wipe|destroy)\b'
    dimension: risk
    weight: 25
    reason: "destructive data operation (evidence)"

  - id: data_loss_evidence
    regex: '(?i)\b(data loss|irreversible|cannot be undone|permanent loss)\b'
    dimension: risk
    weight: 25
    reason: "data loss/irreversibility (evidence)"

  - id: secret_leak_evidence
    regex: '\b(committed|leaked|exposed|pushed)\b.*\b(secret|api.?key|private.?key|credential|password|token)\b|\b(secret|api.?key|private.?key|credential|password|token)\b.*\b(committed|leaked|exposed|pushed)\b'
    dimension: risk
    weight: 25
    reason: "concrete secret leak evidence"

  - id: stuck_agent
    requires: [failed_tests, stack_trace]
    dimension: agent_pressure
    weight: 25
    reason: "agent stuck: failing tests with stack trace"

  - id: greeting
    regex: '^(hi|hello|hey|yo|sup|good (morning|evening)|thanks?|ok|okay|cool|got it)\b'
    dimension: downgrade
    weight: 15
    reason: "greeting/trivial"

  - id: simple_explanation_request
    regex: 'what is|explain (briefly|simply)|short (answer|explanation)|tldr|in one sentence|no code|don''t edit'
    dimension: downgrade
    weight: 10
    reason: "simple explanation / no-edit"

  - id: single_word_db_rename
    regex: '\brename\b.*\b(database|table|column|variable)\b'
    requires_not: [database_topic, production_topic]
    dimension: downgrade
    weight: 8
    reason: "trivial rename, no migration/production context"

# ─────────────────────────────────────────────────────────────
#  Intelligence  — optional enhancement layers
# ─────────────────────────────────────────────────────────────

intelligence:
  enabled: false
  length:
    max_level_without_evidence: medium
  concepts: []
  operation_object:
    proximity_window: 5
    rules: []
  exemplars:
    enabled: false
    path: "/config/exemplars.yaml"
    merge_mode: extend
  similarity:
    mode: hybrid
    bm25:
      enabled: true
      k1: 1.2
      b: 0.75
      min_score: 2.5
      min_margin: 0.18
      max_exemplars_per_level: 5
    hashed_cosine:
      enabled: true
      buckets: 4096
      word_ngrams: [1, 2]
      char_ngrams: [3, 4]
      min_score: 0.22
      min_margin: 0.08
    resolver:
      agreement_bonus: 0.15
      disagreement_fallback_level: hard
      weak_similarity_fallback: policy
      cosine_only_max_level: hard
  uncertainty:
    safer_fallback: true
    exemplar_floor_weight: 0.60
  session:
    enabled: true
    require_session_header: true
    header: "X-Dispatch-Session-Id"
    fallback_key: none
    ttl_minutes: 60
    max_entries: 10000
    escalation:
      hard_after_repeated_failures: 2
      critical_after_repeated_failures: 3
      decay_per_success: 1
  routing_profiles:
    default: balanced
    allow_header_override: false
    header: "X-Dispatch-Profile"
    profiles:
      cheap:
        min_margin: 0.05
        exemplar_floor_weight: 0.70
        floor_reduction: 1
        force_min_level: ""
      balanced:
        min_margin: 0.10
        exemplar_floor_weight: 0.60
        floor_reduction: 0
        force_min_level: ""
      quality:
        min_margin: 0.15
        exemplar_floor_weight: 0.50
        floor_reduction: 0
        force_min_level: medium

# ─────────────────────────────────────────────────────────────
#  Debug / Observability
# ─────────────────────────────────────────────────────────────

debug:
  log_level: "info"
  log_prompts: false
  log_metadata: true
  log_decisions: true
  trace_requests: false
  set_response_headers: true
  request_index_enabled: true
  request_index_size: 500
  feedback_enabled: false
  feedback_path: "/config/feedback.jsonl"

# ─────────────────────────────────────────────────────────────
#  Config Reload
# ─────────────────────────────────────────────────────────────

config_reload:
  enabled: true
  poll_interval_seconds: 3

version: ""
`

const defaultROUTERmd = `# Dispatch — Router Documentation

Dispatch is a bespoke OpenRouter-only complexity router for OpenCode.
It classifies each chat completion request into one of four levels (easy, medium, hard, critical)
and routes it to the configured OpenRouter model for that level.

## Quickstart

1. Copy .env.example to .env and put your OpenRouter API key: OPENROUTER_API_KEY=sk-or-...
2. Start Dispatch — it auto-generates a default config at /config/router.yaml on first run.
3. Define model profiles under "model_profiles", then assign them to levels with "use:".
4. Restart. Done — Dispatch routes by evidence of difficulty, not scary keywords.

Config auto-generates if router.yaml doesn't exist. If you hand-write from scratch, you'll miss the default patterns and exemplars and may introduce YAML bugs. Let Dispatch generate the first config, then customize.

## How Routing Works

The router classifies requests using evidence-based complexity routing, not topic-keyword routing.

**Topic words (auth, security, payment, database, production, deploy, secrets, etc.) are metadata only — they never directly set a route level.**

Routes are determined by:

1. **Structural complexity** — code blocks, diffs, tool calls, strict JSON schemas, multi-file scope, multi-step reasoning.
2. **Concrete failure evidence** — stack traces, failing tests, compile errors, tool errors, repeated failure phrases.
3. **Multi-signal critical gates** — secret leaks, destructive data operations, access violations, transaction impacts, production outages. Every critical gate requires at least 2 concrete evidence signals.
4. **Session escalation** — repeated failures in the same session escalate the route level. First attempt can be cheap, second failure → hard, third → critical.
5. **Length policy** — long prompts and huge contexts are metadata only. Length alone cannot force hard or critical without concrete difficulty evidence.

### Topic Metadata Policy

Words like auth, security, payment, database, production, deploy, secrets, migration, rollback are detected as context metadata but never directly cause hard or critical routing. A prompt mentioning "production database migration" without any failure or destructive action evidence will route at medium at most.

Examples of topic-only prompts that stay easy/medium:
- "fix login button text" → easy/medium
- "payment button CSS" → easy/medium
- "explain security vulnerability in one sentence" → easy/medium
- "add a security section to README" → easy/medium
- "write a migration file" → medium
- "we need a production database migration" → medium

Examples of evidence-based prompts routing hard/critical:
- "migration failed with stack trace" → hard (stack trace evidence)
- "compile error after refactoring three files" → hard (compile error + multi-file)
- "DROP TABLE production data" → critical (destructive action + data context)
- "API key committed to public repo, rotate it" → critical (secret leak + repo + remediation)
- "auth bypass lets users access other accounts" → critical (access violation + impact)

## OpenCode Setup

Point OpenCode at Dispatch with these settings:
- **Base URL**: http://localhost:18087/v1
- **API Key**: placeholder (the real key is on the server side, in the OPENROUTER_API_KEY env var)
- **Model**: dispatch/auto

The router auto-classifies every request and replaces the model with the configured upstream model for that level.
OpenCode may show only dispatch/auto as the available model — the actual upstream model is selected internally.
Use these format for manual overrides:
- Model alias: dispatch/easy, dispatch/medium, dispatch/hard, dispatch/critical, dispatch/auto
- Header: X-Dispatch-Level: easy|medium|hard|critical (takes precedence over model alias)
The selected model is visible in the X-Dispatch-Model response header and in structured logs.

## Config File Map

### openrouter
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| base_url | string | https://openrouter.ai/api/v1 | OpenRouter API base URL |
| api_key_env | string | OPENROUTER_API_KEY | Env var name for the API key |
| validate_models_on_start | bool | false | Validate model IDs against OpenRouter at startup |
| http_referer | string | https://github.com/OpusNano/dispatch | HTTP-Referer header sent upstream (controls OpenRouter app attribution) |
| site_title | string | Dispatch | Optional X-OpenRouter-Title header |

### server
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| listen | string | :18087 | Listen address |
| max_body_size | int | 26214400 | Max request body in bytes (25 MiB) |
| read_timeout_seconds | int | 30 | Read timeout |
| write_timeout_seconds | int | 0 | Write timeout (0 = no timeout, for streaming) |

### model_profiles
Named reusable model profiles. Define your OpenRouter models here — no limit on how many you define.
The YAML key is the profile name (your choice). Only profiles referenced by 'use:' in levels are active.

Each profile has:
- **id** — OpenRouter model ID (e.g., "openai/gpt-4o").
- **provider** — optional provider routing config (order, data_collection, etc.).

Example:

    model_profiles:
      deepseek_flash:
        id: "deepseek/deepseek-v4-flash"
        provider:
          data_collection: "deny"
      deepseek_pro:
        id: "deepseek/deepseek-v4-pro"

### levels
Map of complexity tier to a model profile or inline model. Four levels required: easy, medium, hard, critical.
Multiple levels may reference the same profile with 'use:'.

Each level must set exactly one of:
- **use** — reference a profile name from 'model_profiles'.
- **model** — inline OpenRouter model ID (for quick/simple configs).

Invalid: setting both 'use' and 'model', or setting neither.
If using 'use:', provider settings come from the referenced profile.
If using inline 'model:', provider settings come from that level's 'provider:' field.

Example (profile reference):

    levels:
      easy:
        use: deepseek_flash
      medium:
        use: deepseek_flash
      hard:
        use: deepseek_pro
      critical:
        use: glm_52

Example (inline model):

    levels:
      easy:
        model: "openai/gpt-4o"
        provider:
          data_collection: "deny"

### thresholds
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| easy | float64 | 0 | Minimum total |
| easy_max | float64 | 20 | easy <= total <= easy_max |
| medium_max | float64 | 45 | total <= medium_max -> medium |
| hard_max | float64 | 70 | total <= hard_max -> hard; above -> critical |
| risk_critical_override | float64 | 50 | risk >= this forces critical (only reachable via evidence patterns) |
| risk_hard_floor | float64 | 35 | risk >= this forces at least hard (only reachable via evidence patterns) |
| agent_pressure_critical_override | float64 | 50 | agent_pressure >= this forces critical |

### scoring
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| complexity_cap | float64 | 40 | Max complexity score |
| risk_cap | float64 | 60 | Max risk score |
| agent_pressure_cap | float64 | 30 | Max agent pressure score |
| downgrade_cap | float64 | 30 | Max downgrade score |
| weights.complexity | float64 | 1.0 | Weight multiplier |
| weights.risk | float64 | 1.0 | Weight multiplier |
| weights.agent_pressure | float64 | 0.8 | Weight multiplier |
| weights.downgrade | float64 | -1.0 | Weight multiplier (negative = reduces total) |

### patterns
Each pattern rule has:
- **id** — unique identifier.
- **regex** — case-insensitive regex to match against message text (unless match_tools or match_response_format).
- **dimension** — complexity|risk|agent_pressure|downgrade.
- **weight** — score added when the rule matches.
- **reason** — human-readable explanation (appears in /debug/route).
- **match_tools** — match when request contains "tools" field or tool_call messages.
- **match_response_format** — match when request has "response_format".
- **requires** — list of pattern IDs that must ALL fire for this rule to match (combination rule).
- **requires_not** — list of pattern IDs that must NONE fire for this rule to match.
- **per_match** — if true, added once per regex match (up to "cap").
- **cap** — max total weight from this rule (used with per_match).

Weight conventions:
- Topic/context metadata patterns: weight ~5 (production_topic, security_topic, payment_topic, database_topic, secrets_topic)
- Concrete failure evidence patterns: weight ~15-20 (stack_trace, failed_tests, compile_error, tool_error, repeated_failure)
- Strong evidence patterns: weight ~25 (destructive_action_evidence, data_loss_evidence, secret_leak_evidence, stuck_agent)
- Structural complexity patterns: weight ~8-15 (coding_intent, code_block, diff_patch, multi_file_refactor, multi_step_reasoning)
- Downgrade patterns: weight ~8-15 (greeting, simple_explanation_request, single_word_db_rename)

Topic words never directly force hard or critical. Topic patterns contribute to risk score
at low weight (metadata only). Evidence patterns drive the actual routing decisions.

> **Migration note**: As of this version, topic words such as auth, database, payment, production,
> security, migration, rollback, deploy, and secrets are metadata only and never directly set a route level.
> Old pattern weights (production_risk=15, security_auth=18, database_migration=18) have been replaced
> with low-weight metadata patterns (production_topic=5, security_topic=5, database_topic=5).
> Concrete evidence patterns carry higher weight (stack_trace=18, secret_leak_evidence=25, etc.).

### debug
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| log_level | string | info | error|warn|info|debug |
| log_prompts | bool | false | WARNING: logs full prompts if true |
| log_metadata | bool | true | Log request_id, level, model, scores, reasons, duration, status, stream |
| set_response_headers | bool | true | Add X-Dispatch-* response headers |

### intelligence
Optional enhanced intelligence layer. When enabled, adds:
- Concepts metadata detection
- Operation-object-role extraction
- Exemplar/task similarity routing
- Session-aware escalation
- Routing profiles
- Expanded debug output

The evidence-based core policy (gates, floors, length cap, topic-metadata-only) is always active regardless of intelligence.enabled.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | false | Enable optional intelligence add-ons |

### exemplars
External exemplar file for task similarity routing. Generated on first run at /config/exemplars.yaml.
Set intelligence.exemplars.enabled=true and intelligence.enabled=true to use.

Exemplars are routing examples that compare each request using BM25 + cosine similarity.
Positive exemplars suggest a level. Negative exemplars suppress a level.
Similarity can only RAISE the level (to hard at most). Only explicit override or critical gate produces critical.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | false | Enable exemplar similarity |
| path | string | /config/exemplars.yaml | Path to exemplar file |
| merge_mode | string | extend | extend (built-in + file) | replace (file only) | disabled |

### session
Session-aware escalation. Tracks repeated failure evidence per session and escalates route level.
Only tracks when X-Dispatch-Session-Id header is present. Never stores prompt text.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable session tracking |
| require_session_header | bool | true | Only track when session header present |
| header | string | X-Dispatch-Session-Id | HTTP header to identify sessions |
| ttl_minutes | int | 60 | Session state TTL |
| max_entries | int | 10000 | Max concurrent sessions |
| escalation.hard_after_repeated_failures | int | 2 | Escalate to hard after N failure signals |
| escalation.critical_after_repeated_failures | int | 3 | Escalate to critical after N failure signals |
| escalation.decay_per_success | int | 1 | Reduce failure counter on non-failure request |

### routing_profiles
Config-defined profiles that adjust routing behavior. Profile can only be changed via config (or header if allow_header_override=true). Never bypasses critical gates.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| default | string | balanced | Default profile (cheap|balanced|quality) |
| allow_header_override | bool | false | Allow X-Dispatch-Profile header to select profile |
| header | string | X-Dispatch-Profile | Header for profile override |

## Manual Overrides

OpenCode can force a level via:
- **Model alias**: set "model" to "dispatch/easy|medium|hard|critical" to force that level.
- **Model alias**: set "model" to "dispatch/auto" to classify normally (default for OpenCode).
- **Header**: set "X-Dispatch-Level: easy|medium|hard|critical" header.
  The header **wins** over model alias.

## Provider Merge Policy

When the client sends a "provider" object in the request, it is **merged** with the level's configured provider:
- Client fields remain unless the level config explicitly sets the same field.
- If the level config sets "data_collection", it always overrides the client's value.
- If the level config sets "order", it overrides client "order".
- If the level config sets "allow_fallbacks", it overrides client "allow_fallbacks".
- Fields not set by the level config remain from the client.

## Debug & Visibility

- **POST /debug/route** — classification only, returns level/model/scores/reasons without calling OpenRouter.
  - Response includes request_id for correlation with logs.
- **Response headers**: X-Dispatch-Request-Id, X-Dispatch-Level, X-Dispatch-Model, X-Dispatch-Score-*, X-Dispatch-Reasons.
- **Metadata logs**: structured stdout logs include level, model, scores, reasons, duration.
- **Health**: GET /health returns {"status":"ok"}.
- **Version**: GET /version returns build info.
- **Check config**: run "dispatch --check-config /path/to/router.yaml" to validate without starting.

## Security

- **Never put an API key in this config file.** Use "openrouter.api_key_env" to point to an env var.
- The default env var is OPENROUTER_API_KEY.
- "log_prompts" is false by default. Enabling it logs full prompt content — understand the privacy implications.
- "max_body_size" enforces a request size limit.
- **Messages never modified**: Dispatch only changes the model and provider fields. The original messages, tools, response_format, and all unknown fields are forwarded unchanged. Debug information is never injected into prompt content.

## Content Preservation

Dispatch classifies requests using the active task frame for routing decisions only. The upstream request forwarded to OpenRouter preserves the original messages byte-for-byte. Dispatch never injects request IDs, debug text, route metadata, or headers into message content. Debug information stays in response headers, metadata logs, and /debug endpoints.

## Migration Note

As of this version, topic words such as auth, database, payment, production,
security, migration, rollback, deploy, and secrets are metadata only and never
directly set a route level. Old topic-based pattern weights (production_risk=15,
security_auth=18, database_migration=18) have been replaced with low-weight
metadata patterns (production_topic=5, security_topic=5, database_topic=5).
Concrete evidence patterns carry higher weight (stack_trace=18, compile_error=15,
secret_leak_evidence=25, destructive_action_evidence=25, etc.).

If you have a custom router.yaml with old pattern IDs, update them to the new IDs.
Old critical gates (production_database_migration, database_rollback_production,
data_loss_or_irreversible, auth_bypass_production) have been replaced with
multi-signal evidence-based gates. See the patterns section above for details.

## Instructions for LLMs Editing This Config

If you are an LLM editing this configuration:

1. **Read before editing**: Load router.yaml and this DISPATCH.md.
2. **Validate after editing**: Run "dispatch --check-config --config /path/to/router.yaml".
3. **Never edit**: Set api_key_env to a literal key. Keep it as an env var name.

### Editing Model Profiles and Levels

- **Prefer editing 'model_profiles.<name>.id'** when changing a model — this updates all levels that reference that profile.
- **Prefer adding a new profile** under 'model_profiles' when adding a reusable model.
- **The YAML key IS the profile name** (e.g., 'deepseek_flash'). Do not invent a 'name:' field inside a profile.
- **Do not use 'models.some_name.model'** — the schema is 'model_profiles.<name>.id'.
- **Do not set both 'use' and 'model' on the same level** — they are mutually exclusive.
- **Do not put OpenRouter API keys in this file** — use the 'OPENROUTER_API_KEY' environment variable.
- **Preserve provider settings** unless asked to change them.
- **If provider names are unknown, leave 'order: []'** (empty order means "let OpenRouter choose providers").

### Safe Edits

- Changing 'model_profiles.<name>.id' to a different OpenRouter model ID.
- Adding a new profile under 'model_profiles' and referencing it with 'use:'.
- Adding a new pattern rule with a unique "id", valid "dimension", and a "reason".
- Tweaking "weights" or "thresholds" by small increments (±10%).

### Risky Edits

Explain impact and test carefully:
- Changing caps, override thresholds, or requires/requires_not graphs.
- Removing downgrade rules (they protect cost).
- Setting log_prompts to true.

### Adding a Pattern Rule

        - id: my_rule
          regex: "..."
          dimension: complexity    # one of: complexity, risk, agent_pressure, downgrade
          weight: 5.0
          reason: "explain what this detects"

    For combination rules:

        - id: my_combo
          requires: [rule_a, rule_b]
          requires_not: [downgrade_rule]
          dimension: risk
          weight: 20
          reason: "context-dependent escalation"
7. **Testing changes**: After editing, POST sample requests to /debug/route and verify the selected levels.
`

const DefaultConfigFilename = "router.yaml"

const defaultExemplarsYAML = `# Dispatch exemplars — routing examples for deterministic similarity matching.
# No ML, embeddings, model calls, or external calls involved.
#
# How it works:
#   - Each request is compared to these examples using BM25 + cosine similarity.
#   - Positive exemplars suggest a level. Negative exemplars suppress a level.
#   - Similarity can only RAISE the level, never lower evidence floors or critical gates.
#   - Critical gates and evidence-based floors always win over exemplar signals.
#   - Exemplar similarity capped at hard — only explicit override or critical gate produces critical.
#
# Adding exemplars:
#   - Add real OpenCode prompts as positive exemplars at the correct level.
#   - Add negative exemplars for prompts that look like one level but should be another.
#   - Keep texts concise (1-3 sentences).
#   - Use evidence_required for hard/critical exemplars to require concrete failure/risk evidence.
#   - Run 'dispatch --check-config /path/to/router.yaml' after editing.
#
# merge_mode in router.yaml controls how these interact with built-in exemplars:
#   extend:   built-in + this file (default)
#   replace:  only this file
#   disabled: no exemplar similarity

exemplars:
  - text: "hello world"
    level: easy
    task: greeting
  - text: "what is a closure in javascript"
    level: easy
    task: concept_explanation
    operation: explain
    role: concept
  - text: "explain authentication in one sentence"
    level: easy
    task: concept_explanation
    operation: explain
    object: auth
    role: concept
  - text: "how do I list files in a directory"
    level: easy
    task: simple_question
  - text: "rename a variable in one file"
    level: easy
    task: variable_rename
    operation: rename
    object: variable
    role: variable_name
  - text: "thanks that works"
    level: easy
    task: greeting
  - text: "add a security section in the readme"
    level: easy
    task: docs_edit
    operation: create
    object: security
    role: readme_or_docs
  - text: "what does cannot find name mean in typescript"
    level: easy
    task: concept_explanation
    operation: explain
  - text: "what is the difference between let and const"
    level: easy
    task: concept_explanation
  - text: "explain async await briefly"
    level: easy
    task: concept_explanation
    operation: explain
  - text: "fix login button text"
    level: easy
    task: simple_copy_change
    operation: fix
    object: auth
    role: ui_component
  - text: "auth docs typo"
    level: easy
    task: simple_copy_change
    operation: fix
    object: auth
    role: readme_or_docs

  - text: "write a unit test for this function"
    level: medium
    task: write_test
    operation: create
    object: tests
    role: business_logic
  - text: "fix this compile error undefined reference to foo"
    level: medium
    task: fix_compile_error
    operation: fix
    evidence_required: [compile_error]
  - text: "edit the config to add a new field"
    level: medium
    task: config_edit
    operation: edit
    object: config
    role: config
  - text: "payment button css styling"
    level: medium
    task: ui_styling
    operation: style
    object: payment
    role: ui_component
  - text: "toy local database migration example"
    level: medium
    task: local_demo
    operation: create
    object: database
    role: local_demo
  - text: "rename the database variable to dbconn"
    level: medium
    task: variable_rename
    operation: rename
    object: database
    role: variable_name
  - text: "add a new endpoint to the api"
    level: medium
    task: add_feature
    operation: create
  - text: "write a sql query to join two tables"
    level: medium
    task: write_query
    operation: create
    object: database
    role: data_state
  - text: "fix the typo in the error message"
    level: medium
    task: simple_copy_change
    operation: fix
    role: simple_copy_change
  - text: "add input validation to the form"
    level: medium
    task: add_validation
    operation: edit
  - text: "write a migration file to add a nullable column"
    level: medium
    task: write_migration
    operation: create
    object: database
    role: schema
  - text: "add error handling to the fetch call"
    level: medium
    task: add_error_handling
    operation: edit
  - text: "explain the top error in this huge log no edits"
    level: medium
    task: log_explanation
    operation: explain
    role: log_blob
  - text: "summarize this long readme"
    level: medium
    task: summarization
    operation: summarize
    role: readme_or_docs
  - text: "find the typo in this long file"
    level: medium
    task: find_typo
    operation: debug
    role: simple_copy_change
  - text: "extract the version number from this long config"
    level: medium
    task: extract_info
    operation: read_only
    role: config

  - text: "refactor the architecture across multiple files with a complete redesign"
    level: hard
    task: multi_file_refactor
    operation: refactor
    evidence_required: [multi_file_scope]
  - text: "should we use a monorepo or polyrepo for this project design the architecture"
    level: hard
    task: architecture_decision
    operation: design
    evidence_required: [code_block, multi_file_scope]
  - text: "debug suspicious authentication vulnerability in the login flow with stack trace"
    level: hard
    task: debug_with_evidence
    operation: debug
    object: auth
    role: security_boundary
    evidence_required: [stack_trace]
  - text: "migration failed with stack trace on staging alter table"
    level: hard
    task: failed_migration
    operation: migrate
    object: database
    role: schema
    evidence_required: [stack_trace]
  - text: "the tool call failed twice same error persists exit code 1"
    level: hard
    task: repeated_tool_failure
    operation: debug
    evidence_required: [repeated_failure, tool_error]
  - text: "strict json schema with tool calls for structured output"
    level: hard
    task: strict_structured_output
    evidence_required: [json_schema, tool_calls]
  - text: "compile error after refactoring three files"
    level: hard
    task: compile_error_refactor
    operation: refactor
    evidence_required: [compile_error, multi_file_scope]
  - text: "failing tests with stack trace in the auth module"
    level: hard
    task: failing_tests_with_stack
    operation: debug
    evidence_required: [test_failure, stack_trace]
  - text: "patch failed same error persists build failed"
    level: hard
    task: patch_failure
    operation: fix
    evidence_required: [repeated_failure, compile_error]
  - text: "design the api gateway pattern for our microservices"
    level: hard
    task: architecture_design
    operation: design
    evidence_required: [code_block]
  - text: "restructure the auth module into separate concerns across files"
    level: hard
    task: multi_file_refactor
    operation: refactor
    object: auth
    role: security_boundary
    evidence_required: [multi_file_scope]
  - text: "implement rate limiting with redis and middleware"
    level: hard
    task: implement_feature
    operation: create
  - text: "debug the race condition in the concurrent worker pool"
    level: hard
    task: complex_debug
    operation: debug
  - text: "migrate from rest to graphql with backward compatibility"
    level: hard
    task: complex_migration
    operation: refactor
  - text: "refactor the test suite to use table-driven tests"
    level: hard
    task: test_refactor
    operation: refactor
    evidence_required: [code_block]
  - text: "set up ci cd pipeline with automated testing"
    level: hard
    task: pipeline_setup
    operation: create
    object: deployment
    role: production_system
  - text: "debug the memory leak in the long running process"
    level: hard
    task: complex_debug
    operation: debug
  - text: "implement optimistic concurrency control for the api"
    level: hard
    task: implement_feature
    operation: create
  - text: "optimize the database query causing slow page loads"
    level: hard
    task: optimization
    operation: refactor
    object: database
    role: data_state
  - text: "add comprehensive error handling to the payment service"
    level: hard
    task: add_error_handling
    operation: edit
    object: payment
    role: transaction

  - text: "the production deploy is failing customer-facing service is down with crash logs"
    level: critical
    task: production_outage
    operation: debug
    object: deployment
    role: production_system
    evidence_required: [logs, tool_error]
  - text: "the api key was accidentally committed to the public git repo"
    level: critical
    task: secret_leak
    evidence_required: [secret_leak_evidence]
  - text: "auth bypass lets users access other customer accounts in production"
    level: critical
    task: access_violation
    operation: debug
    object: auth
    role: security_boundary
    evidence_required: [access_impact]
  - text: "drop table in production caused data loss"
    level: critical
    task: destructive_operation
    operation: delete
    object: database
    role: data_state
    evidence_required: [destructive_action]
  - text: "production database rollback needed immediately after failed migration"
    level: critical
    task: production_rollback
    operation: rollback
    object: database
    role: production_system
    evidence_required: [tool_error]
  - text: "payment refund mismatch affecting customers cannot process transactions"
    level: critical
    task: payment_failure
    operation: debug
    object: payment
    role: transaction
    evidence_required: [transaction_impact, customer_impact]
  - text: "security vulnerability exploited in production customers affected"
    level: critical
    task: security_incident
    operation: debug
    object: security
    role: security_boundary
    evidence_required: [access_impact]
  - text: "secrets leaked in the public repository need immediate rotation"
    level: critical
    task: secret_leak
    evidence_required: [secret_leak_evidence]
  - text: "production outage caused by the latest deployment crash loop"
    level: critical
    task: production_outage
    operation: debug
    object: deployment
    role: production_system
    evidence_required: [logs]
  - text: "customer data exposed due to auth bypass in production"
    level: critical
    task: access_violation
    operation: debug
    object: auth
    role: security_boundary
    evidence_required: [access_impact]
  - text: "irreversible data loss from truncate in production database"
    level: critical
    task: destructive_operation
    operation: delete
    object: database
    role: data_state
    evidence_required: [destructive_action]
  - text: "payment system down during peak checkout traffic customers cannot pay"
    level: critical
    task: payment_outage
    operation: debug
    object: payment
    role: transaction
    evidence_required: [transaction_impact, customer_impact]
  - text: "production database schema migration failed and is stuck"
    level: critical
    task: failed_production_migration
    operation: migrate
    object: database
    role: production_system
    evidence_required: [tool_error]
  - text: "critical security patch needed for zero-day vulnerability in production"
    level: critical
    task: security_incident
    operation: fix
    object: security
    role: security_boundary
  - text: "production service crash looping after config change customers impacted"
    level: critical
    task: production_outage
    operation: debug
    object: deployment
    role: production_system
    evidence_required: [logs, customer_impact]

  - text: "explain authentication in one sentence"
    level: hard
    negative: true
    task: concept_explanation
  - text: "explain authentication in one sentence"
    level: critical
    negative: true
    task: concept_explanation
  - text: "rename the database variable to dbconn"
    level: hard
    negative: true
    task: variable_rename
  - text: "rename the database variable to dbconn"
    level: critical
    negative: true
    task: variable_rename
  - text: "add a security section in the readme"
    level: hard
    negative: true
    task: docs_edit
  - text: "add a security section in the readme"
    level: critical
    negative: true
    task: docs_edit
  - text: "payment button css color change"
    level: critical
    negative: true
    task: ui_styling
  - text: "toy local database migration for demo"
    level: critical
    negative: true
    task: local_demo
  - text: "what is a sql injection vulnerability"
    level: critical
    negative: true
    task: concept_explanation
  - text: "what is a sql injection vulnerability"
    level: hard
    negative: true
    task: concept_explanation
  - text: "fix login button text"
    level: hard
    negative: true
    task: simple_copy_change
  - text: "fix login button text"
    level: critical
    negative: true
    task: simple_copy_change
  - text: "production quality code style"
    level: hard
    negative: true
    task: concept_explanation
  - text: "production quality code style"
    level: critical
    negative: true
    task: concept_explanation
  - text: "explain auth bypass no edits"
    level: hard
    negative: true
    task: concept_explanation
  - text: "explain auth bypass no edits"
    level: critical
    negative: true
    task: concept_explanation
  - text: "auth docs typo"
    level: hard
    negative: true
    task: simple_copy_change
  - text: "auth docs typo"
    level: critical
    negative: true
    task: simple_copy_change
  - text: "staging database migration with rollback plan"
    level: critical
    negative: true
    task: write_migration
  - text: "write a migration file to add a nullable column"
    level: critical
    negative: true
    task: write_migration
  - text: "hello world production"
    level: critical
    negative: true
    task: greeting
  - text: "database login auth blah blah but this is a tiny copy change"
    level: hard
    negative: true
    task: simple_copy_change
  - text: "database login auth blah blah but this is a tiny copy change"
    level: critical
    negative: true
    task: simple_copy_change
`
