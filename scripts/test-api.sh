#!/usr/bin/env bash
# LLM Gateway API Test Script
# Usage:
#   ./scripts/test-api.sh                              # Health + models check only
#   ./scripts/test-api.sh chat                         # Chat with default model (gpt-4o-mini)
#   ./scripts/test-api.sh chat qwen-turbo              # Chat with catalog model (auto-resolve provider)
#   ./scripts/test-api.sh chat alibaba:qwen3-0.6b      # Chat with explicit provider:model
#   ./scripts/test-api.sh chat alibaba:qwen3-0.6b "Hi" # Chat with provider:model and prompt
#   ./scripts/test-api.sh stream alibaba:qwen-turbo     # Stream with provider:model
#   ./scripts/test-api.sh responses gpt-4o              # Responses API
#   ./scripts/test-api.sh responses-stream              # Responses API (streaming)
#   ./scripts/test-api.sh errors                        # Error scenario tests
#   ./scripts/test-api.sh all                           # Run all tests
#
# Environment:
#   GATEWAY_URL  - Gateway base URL (default: http://localhost:8080)
#
# Prerequisites:
#   1. Configure config/.env with your API keys
#   2. Start the server: go run cmd/server/main.go --env config/.env

set -eo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8088}"
MODE="${1:-}"
MODEL_ARG="${2:-gpt-4o-mini}"
CONTENT="${3:-Say hello in one word.}"
MAX_TOKENS="${MAX_TOKENS:-20}"

# Parse provider:model format (e.g. "alibaba:qwen3-0.6b")
PROVIDER=""
MODEL="$MODEL_ARG"
if [[ "$MODEL_ARG" == *:* ]]; then
    PROVIDER="${MODEL_ARG%%:*}"
    MODEL="${MODEL_ARG#*:}"
fi

# Build the provider JSON fragment (empty string if no provider specified)
provider_json() {
    if [[ -n "$PROVIDER" ]]; then
        echo "\"provider\": \"$PROVIDER\","
    fi
}

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
    info "Testing chat: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Prompt: $content"

    local resp code body
    resp=$(curl -s -w "\n%{http_code}" --max-time 60 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
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
        echo "$body" | python3 -c "import sys,json; print(json.dumps(json.load(sys.stdin),indent=4,ensure_ascii=False))" 2>/dev/null || echo "  $body"
    fi
    echo
}

# --- Chat Completion (streaming) ---
test_chat_stream() {
    local model="$1"
    local content="$2"
    info "Testing stream: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Prompt: $content"
    echo -n "  Response: "

    local exit_code=0
    curl -s -N --max-time 60 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
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

# --- Responses API (non-streaming) ---
test_responses() {
    local model="$1"
    local content="$2"
    info "Testing responses API: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Prompt: $content"

    local resp code body
    resp=$(curl -s -w "\n%{http_code}" --max-time 60 "$BASE_URL/v1/responses" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
            \"model\": \"$model\",
            \"input\": \"$content\",
            \"max_output_tokens\": $MAX_TOKENS
        }" 2>&1) || true

    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')

    if [[ "$code" == "200" ]]; then
        local reply status
        reply=$(echo "$body" | python3 -c "
import sys, json
resp = json.load(sys.stdin)
output = resp.get('output', [])
for item in output:
    if item.get('type') == 'message':
        for part in item.get('content', []):
            if part.get('type') == 'output_text':
                print(part.get('text', ''))
                break
        break
else:
    print('(no message output)')
" 2>/dev/null || echo "(parse error)")
        status=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "unknown")
        ok "Responses API ($model) [status=$status]:"
        echo "  $reply"
        # Show usage
        echo "$body" | python3 -c "
import sys, json
u = json.load(sys.stdin).get('usage', {})
if u:
    print(f\"  [tokens: input={u.get('input_tokens',0)}, output={u.get('output_tokens',0)}, total={u.get('total_tokens',0)}]\")
" 2>/dev/null || true
    else
        fail "Responses API failed ($model) → HTTP $code"
        echo "  Response:"
        echo "$body" | python3 -c "import sys,json; print(json.dumps(json.load(sys.stdin),indent=4,ensure_ascii=False))" 2>/dev/null || echo "  $body"
    fi
    echo
}

# --- Responses API (streaming) ---
test_responses_stream() {
    local model="$1"
    local content="$2"
    info "Testing responses stream: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Prompt: $content"
    echo -n "  Response: "

    local exit_code=0
    curl -s -N --max-time 60 "$BASE_URL/v1/responses" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
            \"model\": \"$model\",
            \"input\": \"$content\",
            \"max_output_tokens\": $MAX_TOKENS,
            \"stream\": true
        }" 2>/dev/null | python3 -u -c "
import sys, json
got_content = False
for line in sys.stdin:
    line = line.strip()
    if not line.startswith('data: '):
        # Also check for 'event:' lines (Responses API uses named events)
        continue
    data = line[6:]
    if data == '[DONE]':
        break
    try:
        d = json.loads(data)
        event_type = d.get('type', '')
        # Check for error event
        if event_type == 'error' or 'error' in d:
            err = d.get('error', {})
            msg = err.get('message', err) if isinstance(err, dict) else err
            print(f'\nError: {msg}', file=sys.stderr)
            sys.exit(1)
        # Extract text delta from content_part.delta events
        if event_type == 'response.content_part.delta':
            delta = d.get('delta', {})
            text = delta.get('text', '')
            if text:
                print(text, end='', flush=True)
                got_content = True
    except Exception:
        pass
if not got_content:
    print('(no content received)', end='')
    sys.exit(1)
" || exit_code=$?

    echo
    if [[ "$exit_code" -eq 0 ]]; then
        ok "Responses stream completed ($model)"
    else
        fail "Responses stream failed ($model)"
    fi
    echo
}

# --- Error Scenario Tests ---
test_errors() {
    local passed=0
    local failed=0

    info "Running error scenario tests..."
    echo

    # Test 1: Invalid request body → 400
    info "[Error 1/5] Invalid request body → expect 400"
    local resp code
    resp=$(curl -s -w "\n%{http_code}" --max-time 10 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "this is not json" 2>&1) || true
    code=$(echo "$resp" | tail -1)
    if [[ "$code" == "400" ]]; then
        ok "Invalid body → 400"; ((passed++))
    else
        fail "Invalid body → expected 400, got $code"; ((failed++))
    fi

    # Test 2: Method Not Allowed → 405
    info "[Error 2/5] GET /v1/chat/completions → expect 405"
    resp=$(curl -s -w "\n%{http_code}" --max-time 10 "$BASE_URL/v1/chat/completions" 2>&1) || true
    code=$(echo "$resp" | tail -1)
    if [[ "$code" == "405" ]]; then
        ok "GET on POST-only endpoint → 405"; ((passed++))
    else
        fail "GET on POST-only endpoint → expected 405, got $code"; ((failed++))
    fi

    # Test 3: Method Not Allowed on Responses API → 405
    info "[Error 3/5] GET /v1/responses → expect 405"
    resp=$(curl -s -w "\n%{http_code}" --max-time 10 "$BASE_URL/v1/responses" 2>&1) || true
    code=$(echo "$resp" | tail -1)
    if [[ "$code" == "405" ]]; then
        ok "GET on /v1/responses → 405"; ((passed++))
    else
        fail "GET on /v1/responses → expected 405, got $code"; ((failed++))
    fi

    # Test 4: Invalid model → expect error (typically 404 or provider error)
    info "[Error 4/5] Non-existent model → expect error"
    resp=$(curl -s -w "\n%{http_code}" --max-time 15 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "this-model-does-not-exist-xyz-99999",
            "messages": [{"role": "user", "content": "test"}]
        }' 2>&1) || true
    code=$(echo "$resp" | tail -1)
    local body
    body=$(echo "$resp" | sed '$d')
    if [[ "$code" -ge 400 ]]; then
        ok "Non-existent model → $code (error as expected)"
        echo "$body" | python3 -c "
import sys, json
try:
    e = json.load(sys.stdin).get('error', {})
    print(f\"  Error: {e.get('message', 'unknown')}\")
except: pass" 2>/dev/null || true
        ((passed++))
    else
        fail "Non-existent model → expected 4xx, got $code"; ((failed++))
    fi

    # Test 5: Empty messages array → expect error
    info "[Error 5/5] Empty messages → expect error"
    resp=$(curl -s -w "\n%{http_code}" --max-time 15 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "gpt-4o-mini",
            "messages": []
        }' 2>&1) || true
    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')
    if [[ "$code" -ge 400 ]]; then
        ok "Empty messages → $code (error as expected)"
        ((passed++))
    else
        # Some providers may accept empty messages; treat 200 as a warning
        warn "Empty messages → $code (some providers may accept this)"
        ((passed++))
    fi

    echo
    info "Error tests: $passed passed, $failed failed (total 5)"
    if [[ "$failed" -gt 0 ]]; then
        return 1
    fi
}

# --- Help ---
show_help() {
    echo "LLM Gateway Test Script"
    echo ""
    echo "Usage:"
    echo "  $0                                        # Health check + list models"
    echo "  $0 chat [provider:]model [prompt]         # Non-streaming chat"
    echo "  $0 stream [provider:]model [prompt]       # Streaming chat"
    echo "  $0 responses [provider:]model [prompt]    # Responses API (non-streaming)"
    echo "  $0 responses-stream [provider:]model [prompt]  # Responses API (streaming)"
    echo "  $0 errors                                 # Error scenario tests"
    echo "  $0 all [provider:]model [prompt]          # Run all tests"
    echo ""
    echo "The [provider:]model argument supports two formats:"
    echo "  model-name          Use a model from the catalog (auto-resolve provider)"
    echo "  provider:model      Explicitly specify provider (required for unlisted models)"
    echo ""
    echo "Examples:"
    echo "  $0 chat qwen-turbo                        # Catalog model (provider auto-resolved)"
    echo "  $0 chat alibaba:qwen3-0.6b                # Explicit provider + unlisted model"
    echo "  $0 stream alibaba:qwen-turbo 'Write a haiku'"
    echo "  $0 chat openai:gpt-4o 'Tell me a joke'"
    echo "  $0 responses gpt-4o 'What is 2+2?'"
    echo "  $0 errors"
    echo "  $0 all alibaba:qwen-turbo"
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
    responses)
        test_responses "$MODEL" "$CONTENT"
        ;;
    responses-stream)
        test_responses_stream "$MODEL" "$CONTENT"
        ;;
    errors)
        test_errors
        ;;
    all)
        test_chat "$MODEL" "$CONTENT"
        test_chat_stream "$MODEL" "$CONTENT"
        test_responses "$MODEL" "$CONTENT"
        test_responses_stream "$MODEL" "$CONTENT"
        test_errors
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
