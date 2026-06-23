#!/bin/sh
# Dispatch smoke test — tests local endpoints without a real OpenRouter API key.
# Usage: ./scripts/smoke.sh [base_url]
# Default base_url: http://localhost:18087

set -e

BASE_URL="${1:-http://localhost:18087}"
PASS=0
FAIL=0

ok() {
    printf "  [PASS] %s\n" "$1"
    PASS=$((PASS + 1))
}

nok() {
    printf "  [FAIL] %s\n" "$1"
    FAIL=$((FAIL + 1))
}

echo "Dispatch smoke test against $BASE_URL"
echo ""

# 1. Health
echo "1. GET /health"
resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" 2>/dev/null)
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | head -1)
if [ "$code" = "200" ] && echo "$body" | grep -q '"ok"'; then
    ok "health returns 200 with status ok"
else
    nok "health: code=$code body=$body"
fi

# 2. Version
echo "2. GET /version"
resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/version" 2>/dev/null)
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | head -1)
if [ "$code" = "200" ] && echo "$body" | grep -q '"version"'; then
    ok "version returns 200 with version field"
else
    nok "version: code=$code body=$body"
fi

# 3. Debug route
echo "3. POST /debug/route"
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/debug/route" \
    -H "Content-Type: application/json" \
    -d '{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}]}' 2>/dev/null)
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | head -1)
if [ "$code" = "200" ] && echo "$body" | grep -q '"level"'; then
    ok "debug/route returns 200 with level field"
else
    nok "debug/route: code=$code body=$body"
fi

# 4. Debug route with content array
echo "4. POST /debug/route (content array)"
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/debug/route" \
    -H "Content-Type: application/json" \
    -d '{"model":"dispatch/auto","messages":[{"role":"user","content":[{"type":"text","text":"production database migration rollback"}]}]}' 2>/dev/null)
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | head -1)
if [ "$code" = "200" ] && echo "$body" | grep -q '"critical"'; then
    ok "debug/route classifies content array as critical"
else
    nok "debug/route (content array): code=$code body=$body"
fi

# 5. Invalid JSON returns 400
echo "5. POST /v1/chat/completions (invalid JSON)"
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{not valid json}' 2>/dev/null)
code=$(echo "$resp" | tail -1)
if [ "$code" = "400" ]; then
    ok "invalid JSON returns 400"
else
    nok "invalid JSON: code=$code (expected 400)"
fi

# 6. Request ID header
echo "6. Request ID header"
rid=$(curl -s -D - -o /dev/null -X POST "$BASE_URL/debug/route" \
    -H "Content-Type: application/json" \
    -d '{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}]}' 2>/dev/null | grep -i "X-Dispatch-Request-Id" | tr -d '\r')
if [ -n "$rid" ]; then
    ok "request ID header present: $rid"
else
    nok "request ID header missing"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ] && exit 0 || exit 1
