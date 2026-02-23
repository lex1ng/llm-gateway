#!/usr/bin/env bash
# LLM Gateway API Test Script
# Usage:
#   ./scripts/test-api.sh                              # Health + models check only
#   ./scripts/test-api.sh chat                         # Chat with default model (gpt-4o-mini)
#   ./scripts/test-api.sh chat gpt-5                   # Chat with specific model
#   ./scripts/test-api.sh chat gpt-5 "Hello world"    # Chat with specific model and prompt
#   ./scripts/test-api.sh stream                       # Stream with default model
#   ./scripts/test-api.sh stream gpt-5                 # Stream with specific model
#   ./scripts/test-api.sh stream gpt-5 "Tell a joke"  # Stream with specific model and prompt
#
# Environment:
#   GATEWAY_URL  - Gateway base URL (default: http://localhost:8080)
#
# Prerequisites:
#   1. Configure config/.env with your API keys
#   2. Start the server: go run cmd/server/main.go --env config/.env

set -eo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
MODE="${1:-}"
MODEL="${2:-gpt-4o-mini}"
CONTENT="${3:-Say hello in one word.}"
MAX_TOKENS="${MAX_TOKENS:-20}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[PASS]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

# --- Health Check ---
test_health() {
    info "Testing health endpoint..."
    local resp code body
    resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" 2>&1) || true
    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')

    if [[ "$code" == "200" ]]; then
        ok "GET /health → 200: $body"
    else
        fail "GET /health → $code"
        echo "  Response: $body"
        echo "  Is the server running at $BASE_URL?"
        exit 1
    fi
}

# --- List Models ---
test_list_models() {
    info "Listing available models..."
    local resp count
    resp=$(curl -s "$BASE_URL/v1/models" 2>&1) || true
    count=$(echo "$resp" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null || echo "0")

    if [[ "$count" -gt 0 ]]; then
        ok "GET /v1/models → $count models in catalog"
        echo "$resp" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', [])
for m in data[:10]:  # Show first 10
    print(f\"  - {m['id']} (owned_by: {m['owned_by']})\")" 2>/dev/null
        if [[ "$count" -gt 10 ]]; then
            echo "  ... and $((count - 10)) more"
        fi
    else
        warn "GET /v1/models → no models in catalog (passthrough still works)"
    fi
    echo
}

# --- Chat Completion (non-streaming) ---
test_chat() {
    local model="$1"
    local content="$2"
    info "Testing chat: model=$model"
    echo "  Prompt: $content"

    local resp code body
    resp=$(curl -s -w "\n%{http_code}" --max-time 60 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$model\",
            \"messages\": [{\"role\": \"user\", \"content\": \"$content\"}],
            \"max_tokens\": $MAX_TOKENS
        }" 2>&1) || true

    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')

    if [[ "$code" == "200" ]]; then
        local reply
        reply=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin)['choices'][0]['message']['content'])" 2>/dev/null || echo "(parse error)")
        ok "Chat response ($model):"
        echo "  $reply"
        # Show usage
        echo "$body" | python3 -c "
import sys, json
u = json.load(sys.stdin).get('usage', {})
if u:
    print(f\"  [tokens: prompt={u.get('prompt_tokens',0)}, completion={u.get('completion_tokens',0)}, total={u.get('total_tokens',0)}]\")" 2>/dev/null || true
    else
        fail "Chat failed ($model) → HTTP $code"
        echo "  Response:"
        echo "$body" | python3 -m json.tool 2>/dev/null || echo "  $body"
    fi
    echo
}

# --- Chat Completion (streaming) ---
test_chat_stream() {
    local model="$1"
    local content="$2"
    info "Testing stream: model=$model"
    echo "  Prompt: $content"
    echo -n "  Response: "

    local exit_code=0
    curl -s -N --max-time 60 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$model\",
            \"messages\": [{\"role\": \"user\", \"content\": \"$content\"}],
            \"max_tokens\": $MAX_TOKENS,
            \"stream\": true
        }" 2>/dev/null | python3 -u -c "
import sys, json
got_content = False
for line in sys.stdin:
    line = line.strip()
    if not line.startswith('data: '):
        continue
    data = line[6:]
    if data == '[DONE]':
        break
    try:
        d = json.loads(data)
        # Check for error
        if 'error' in d:
            print(f\"\\nError: {d['error'].get('message', d['error'])}\", file=sys.stderr)
            sys.exit(1)
        c = d.get('choices',[{}])[0].get('delta',{}).get('content','')
        if c:
            print(c, end='', flush=True)
            got_content = True
    except Exception as e:
        pass
if not got_content:
    print('(no content received)', end='')
    sys.exit(1)
" || exit_code=$?

    echo
    if [[ "$exit_code" -eq 0 ]]; then
        ok "Stream completed ($model)"
    else
        fail "Stream failed ($model)"
    fi
    echo
}

# --- Help ---
show_help() {
    echo "LLM Gateway Test Script"
    echo ""
    echo "Usage:"
    echo "  $0                              # Health check + list models"
    echo "  $0 chat [model] [prompt]        # Non-streaming chat"
    echo "  $0 stream [model] [prompt]      # Streaming chat"
    echo ""
    echo "Examples:"
    echo "  $0 chat                         # Chat with gpt-4o-mini"
    echo "  $0 chat gpt-5                   # Chat with gpt-5 (passthrough)"
    echo "  $0 chat gpt-5 'Tell me a joke'  # Chat with custom prompt"
    echo "  $0 stream claude-3-5-sonnet-20241022 'Write a haiku'"
    echo ""
    echo "Environment:"
    echo "  GATEWAY_URL   Base URL (default: http://localhost:8080)"
    echo "  MAX_TOKENS    Max tokens (default: 20)"
}

# --- Main ---
echo "============================================"
echo "  LLM Gateway API Test"
echo "  Target: $BASE_URL"
echo "============================================"
echo

test_health
test_list_models

case "$MODE" in
    chat)
        test_chat "$MODEL" "$CONTENT"
        ;;
    stream)
        test_chat_stream "$MODEL" "$CONTENT"
        ;;
    help|-h|--help)
        show_help
        ;;
    "")
        info "No mode specified. Run '$0 help' for usage."
        ;;
    *)
        fail "Unknown mode: $MODE"
        show_help
        exit 1
        ;;
esac

echo "============================================"
echo "  Done"
echo "============================================"
