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
#   ./scripts/test-api.sh embed alibaba:text-embedding-v3       # Embedding with provider:model
#   ./scripts/test-api.sh embed text-embedding-v3 "自定义文本"  # Embedding with custom text
#   ./scripts/test-api.sh errors                        # Error scenario tests
#   ./scripts/test-api.sh all                           # Run all tests
#
# Environment:
#   GATEWAY_URL  - Gateway base URL (default: http://localhost:8088)
#
# Prerequisites:
#   1. Configure config/.env with your API keys
#   2. Start the server: go run cmd/server/main.go --env config/.env

set -eo pipefail

BASE_URL="${GATEWAY_URL:-http://localhost:8088}"
MODE="${1:-}"
MODEL_ARG="${2:-gpt-4o-mini}"
CONTENT="${3:-介绍朱元璋，用两句话}"
MAX_TOKENS="${MAX_TOKENS:-100}"

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

# Reliable HTTP request: writes body to temp file, stderr to err file, returns HTTP code
# Usage: code=$(do_curl TMPFILE curl_args...)
do_curl() {
    local tmpfile="$1"; shift
    local errfile="${tmpfile}.err"
    local http_code
    http_code=$(curl -sS -o "$tmpfile" -w "%{http_code}" "$@" 2>"$errfile") || http_code="000"
    # If HTTP 000 (connection failure), append curl error to body file for diagnosis
    if [[ "$http_code" == "000" ]]; then
        local curl_err
        curl_err=$(cat "$errfile" 2>/dev/null)
        if [[ -n "$curl_err" ]]; then
            echo "$curl_err" >> "$tmpfile"
        fi
    fi
    rm -f "$errfile"
    echo "$http_code"
}

# --- Health Check ---
test_health() {
    info "Testing health endpoint..."
    local tmpfile code
    tmpfile=$(mktemp)
    code=$(do_curl "$tmpfile" --max-time 10 "$BASE_URL/health")

    if [[ "$code" == "200" ]]; then
        ok "GET /health → 200: $(cat "$tmpfile")"
    else
        fail "GET /health → $code"
        echo "  Response: $(cat "$tmpfile")"
        echo "  Is the server running at $BASE_URL?"
        rm -f "$tmpfile"
        exit 1
    fi
    rm -f "$tmpfile"
}

# --- List Models ---
test_list_models() {
    info "Listing available models..."
    local tmpfile code
    tmpfile=$(mktemp)
    code=$(do_curl "$tmpfile" --max-time 10 "$BASE_URL/v1/models")

    if [[ "$code" != "200" ]]; then
        warn "GET /v1/models → HTTP $code"
        rm -f "$tmpfile"
        echo
        return
    fi

    python3 -c "
import sys, json
with open('$tmpfile') as f:
    resp = json.load(f)
data = resp.get('data', [])
if data:
    print(f'  \033[0;32m[PASS]\033[0m GET /v1/models → {len(data)} models')
    for m in data[:10]:
        print(f\"  - {m['id']} (owned_by: {m.get('owned_by','?')})\")
    if len(data) > 10:
        print(f'  ... and {len(data)-10} more')
else:
    print('  \033[1;33m[WARN]\033[0m GET /v1/models → no models in catalog (passthrough still works)')
" 2>/dev/null || warn "GET /v1/models → parse error"

    rm -f "$tmpfile"
    echo
}

# --- Chat Completion (non-streaming) ---
test_chat() {
    local model="$1"
    local content="$2"
    info "Testing chat: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Prompt: $content"

    local tmpfile code
    tmpfile=$(mktemp)
    code=$(do_curl "$tmpfile" --max-time 120 \
        "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
            \"model\": \"$model\",
            \"messages\": [{\"role\": \"user\", \"content\": \"$content\"}],
            \"max_tokens\": $MAX_TOKENS
        }")

    if [[ "$code" == "200" ]]; then
        python3 -c "
import sys, json
with open('$tmpfile') as f:
    resp = json.load(f)
choices = resp.get('choices', [])
if choices:
    msg = choices[0].get('message', {})
    content = msg.get('content', '')
    reasoning = msg.get('reasoning_content', '')
    if reasoning:
        print(f'  \033[0;36m[思考过程]\033[0m')
        for line in reasoning.strip().split('\n')[:10]:
            print(f'  {line}')
        if len(reasoning.strip().split('\n')) > 10:
            print(f'  ... (truncated)')
    if content:
        print(f'  \033[0;32m[PASS]\033[0m Chat response ($model):')
        for line in content.strip().split('\n'):
            print(f'  {line}')
    elif reasoning:
        print(f'  \033[1;33m[WARN]\033[0m Chat content is empty (all tokens used for reasoning, try increasing MAX_TOKENS)')
    else:
        print(f'  \033[1;33m[WARN]\033[0m Chat response ($model): (empty content)')
        print(f'  Raw response:')
        print(f'  {json.dumps(resp, indent=2, ensure_ascii=False)[:500]}')
else:
    print(f'  \033[0;31m[FAIL]\033[0m Chat response ($model): no choices in response')
    print(f'  Raw: {json.dumps(resp, indent=2, ensure_ascii=False)[:500]}')

u = resp.get('usage', {})
if u:
    print(f'  [tokens: prompt={u.get(\"prompt_tokens\",0)}, completion={u.get(\"completion_tokens\",0)}, total={u.get(\"total_tokens\",0)}]')
" 2>/dev/null || {
            fail "Chat response parse error ($model)"
            echo "  Raw response:"
            cat "$tmpfile" | head -c 500
            echo
        }
    else
        if [[ "$code" == "000" ]]; then
            fail "Chat failed ($model) → connection error (HTTP 000)"
            echo "  Curl error: $(cat "$tmpfile" 2>/dev/null || echo 'unknown')"
            echo "  Possible causes:"
            echo "    - Server WriteTimeout too short (default 30s, reasoning models need longer)"
            echo "    - Network timeout / connection reset"
            echo "    - Server crashed or OOM killed"
            echo "  Check server logs: journalctl -u llm-gateway --since '1 min ago'"
        else
            fail "Chat failed ($model) → HTTP $code"
            echo "  Response:"
            python3 -c "
import json
with open('$tmpfile') as f:
    print(json.dumps(json.load(f), indent=2, ensure_ascii=False))
" 2>/dev/null || cat "$tmpfile"
        fi
    fi

    rm -f "$tmpfile"
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
    local errfile=$(mktemp)
    curl -sS -N --max-time 120 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
            \"model\": \"$model\",
            \"messages\": [{\"role\": \"user\", \"content\": \"$content\"}],
            \"max_tokens\": $MAX_TOKENS,
            \"stream\": true
        }" 2>"$errfile" | python3 -u -c "
import sys, json
got_content = False
in_reasoning = False
for line in sys.stdin:
    line = line.strip()
    if not line.startswith('data: '):
        continue
    data = line[6:]
    if data == '[DONE]':
        break
    try:
        d = json.loads(data)
        if 'error' in d:
            print(f\"\nError: {d['error'].get('message', d['error'])}\", file=sys.stderr)
            sys.exit(1)
        delta = d.get('choices',[{}])[0].get('delta',{})
        # Reasoning content (thinking)
        rc = delta.get('reasoning_content','')
        if rc:
            if not in_reasoning:
                print('\n  \033[0;36m[思考中...]\033[0m ', end='', flush=True)
                in_reasoning = True
            print(rc, end='', flush=True)
            got_content = True
        # Actual content
        c = delta.get('content','')
        if c:
            if in_reasoning:
                print('\n  \033[0;32m[回答]\033[0m ', end='', flush=True)
                in_reasoning = False
            print(c, end='', flush=True)
            got_content = True
    except Exception:
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
        local curl_err
        curl_err=$(cat "$errfile" 2>/dev/null)
        if [[ -n "$curl_err" ]]; then
            echo "  Curl error: $curl_err"
        fi
    fi
    rm -f "$errfile"
    echo
}

# --- Responses API (non-streaming) ---
test_responses() {
    local model="$1"
    local content="$2"
    info "Testing responses API: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Prompt: $content"

    local tmpfile code
    tmpfile=$(mktemp)
    code=$(do_curl "$tmpfile" --max-time 60 \
        "$BASE_URL/v1/responses" \
        -H "Content-Type: application/json" \
        -d "{
            $(provider_json)
            \"model\": \"$model\",
            \"input\": \"$content\",
            \"max_output_tokens\": $MAX_TOKENS
        }")

    if [[ "$code" == "200" ]]; then
        python3 -c "
import json
with open('$tmpfile') as f:
    resp = json.load(f)
output = resp.get('output', [])
status = resp.get('status', 'unknown')
text = ''
for item in output:
    if item.get('type') == 'message':
        for part in item.get('content', []):
            if part.get('type') == 'output_text':
                text = part.get('text', '')
                break
        break
if text:
    print(f'  \033[0;32m[PASS]\033[0m Responses API ($model) [status={status}]:')
    for line in text.strip().split('\n'):
        print(f'  {line}')
else:
    print(f'  \033[1;33m[WARN]\033[0m Responses API ($model): (no message output)')
    print(f'  Raw: {json.dumps(resp, indent=2, ensure_ascii=False)[:500]}')

u = resp.get('usage', {})
if u:
    print(f'  [tokens: input={u.get(\"input_tokens\",0)}, output={u.get(\"output_tokens\",0)}, total={u.get(\"total_tokens\",0)}]')
" 2>/dev/null || {
            fail "Responses API parse error ($model)"
            cat "$tmpfile" | head -c 500
            echo
        }
    else
        fail "Responses API failed ($model) → HTTP $code"
        echo "  Response:"
        python3 -c "
import json
with open('$tmpfile') as f:
    print(json.dumps(json.load(f), indent=2, ensure_ascii=False))
" 2>/dev/null || cat "$tmpfile"
    fi

    rm -f "$tmpfile"
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
        continue
    data = line[6:]
    if data == '[DONE]':
        break
    try:
        d = json.loads(data)
        event_type = d.get('type', '')
        if event_type == 'error' or 'error' in d:
            err = d.get('error', {})
            msg = err.get('message', err) if isinstance(err, dict) else err
            print(f'\nError: {msg}', file=sys.stderr)
            sys.exit(1)
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

# --- Embeddings ---
test_embedding() {
    local model="$1"
    local text="$2"
    info "Testing embedding: ${PROVIDER:+provider=$PROVIDER }model=$model"
    echo "  Input: $text"

    # Build JSON body
    local json_body
    json_body=$(python3 -c "
import json
d = {'model': '$model', 'input': ['$text', 'second sentence for comparison']}
provider = '$PROVIDER'
if provider:
    d['provider'] = provider
print(json.dumps(d, ensure_ascii=False))
")

    local tmpfile code
    tmpfile=$(mktemp)
    code=$(do_curl "$tmpfile" --max-time 30 \
        "$BASE_URL/v1/embeddings" \
        -H "Content-Type: application/json" \
        -d "$json_body")

    if [[ "$code" == "200" ]]; then
        python3 -c "
import json
with open('$tmpfile') as f:
    resp = json.load(f)
data = resp.get('data', [])
model = resp.get('model', 'unknown')
usage = resp.get('usage', {})
print(f'  Model: {model}')
print(f'  Vectors: {len(data)}')
for i, item in enumerate(data):
    vec = item.get('embedding', [])
    dims = len(vec)
    preview = vec[:3] if dims > 3 else vec
    preview_str = ', '.join(f'{v:.6f}' for v in preview)
    print(f'  [{i}] {dims} dims → [{preview_str}, ...]')
if usage:
    print(f'  [tokens: prompt={usage.get(\"prompt_tokens\",0)}, total={usage.get(\"total_tokens\",0)}]')
" 2>/dev/null && ok "Embedding completed ($model)" || {
            fail "Embedding parse error ($model)"
            cat "$tmpfile" | head -c 500
            echo
        }
    else
        fail "Embedding failed ($model) → HTTP $code"
        echo "  Response:"
        python3 -c "
import json
with open('$tmpfile') as f:
    print(json.dumps(json.load(f), indent=2, ensure_ascii=False))
" 2>/dev/null || cat "$tmpfile"
    fi

    rm -f "$tmpfile"
    echo
}

# --- Error Scenario Tests ---
test_errors() {
    local passed=0
    local failed=0

    info "Running error scenario tests..."
    echo

    local tmpfile code
    tmpfile=$(mktemp)

    # Test 1: Invalid request body → 400
    info "[Error 1/5] Invalid request body → expect 400"
    code=$(do_curl "$tmpfile" --max-time 10 \
        "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "this is not json")
    if [[ "$code" == "400" ]]; then
        ok "Invalid body → 400"; ((passed++))
    else
        fail "Invalid body → expected 400, got $code"; ((failed++))
    fi

    # Test 2: Method Not Allowed → 405
    info "[Error 2/5] GET /v1/chat/completions → expect 405"
    code=$(do_curl "$tmpfile" --max-time 10 "$BASE_URL/v1/chat/completions")
    if [[ "$code" == "405" ]]; then
        ok "GET on POST-only endpoint → 405"; ((passed++))
    else
        fail "GET on POST-only endpoint → expected 405, got $code"; ((failed++))
    fi

    # Test 3: Method Not Allowed on Responses API → 405
    info "[Error 3/5] GET /v1/responses → expect 405"
    code=$(do_curl "$tmpfile" --max-time 10 "$BASE_URL/v1/responses")
    if [[ "$code" == "405" ]]; then
        ok "GET on /v1/responses → 405"; ((passed++))
    else
        fail "GET on /v1/responses → expected 405, got $code"; ((failed++))
    fi

    # Test 4: Invalid model → expect error (typically 404 or provider error)
    info "[Error 4/5] Non-existent model → expect error"
    code=$(do_curl "$tmpfile" --max-time 15 \
        "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model": "this-model-does-not-exist-xyz-99999", "messages": [{"role": "user", "content": "test"}]}')
    if [[ "$code" -ge 400 ]]; then
        ok "Non-existent model → $code (error as expected)"
        python3 -c "
import json
with open('$tmpfile') as f:
    e = json.load(f).get('error', {})
    print(f\"  Error: {e.get('message', 'unknown')}\")
" 2>/dev/null || true
        ((passed++))
    else
        fail "Non-existent model → expected 4xx, got $code"; ((failed++))
    fi

    # Test 5: Empty messages array → expect error
    info "[Error 5/5] Empty messages → expect error"
    code=$(do_curl "$tmpfile" --max-time 15 \
        "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model": "gpt-4o-mini", "messages": []}')
    if [[ "$code" -ge 400 ]]; then
        ok "Empty messages → $code (error as expected)"
        ((passed++))
    else
        warn "Empty messages → $code (some providers may accept this)"
        ((passed++))
    fi

    rm -f "$tmpfile"

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
    echo "  $0 embed [provider:]model [text]          # Embeddings"
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
    echo "  $0 embed alibaba:text-embedding-v3        # Embedding with explicit provider"
    echo "  $0 embed alibaba:text-embedding-v3 '自定义文本'"
    echo "  $0 errors"
    echo "  $0 all alibaba:qwen-turbo"
    echo ""
    echo "Environment:"
    echo "  GATEWAY_URL   Base URL (default: http://localhost:8088)"
    echo "  MAX_TOKENS    Max tokens (default: 100)"
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
    embed|embedding|embeddings)
        test_embedding "$MODEL" "$CONTENT"
        ;;
    errors)
        test_errors
        ;;
    all)
        test_chat "$MODEL" "$CONTENT"
        test_chat_stream "$MODEL" "$CONTENT"
        test_responses "$MODEL" "$CONTENT"
        test_responses_stream "$MODEL" "$CONTENT"
        test_embedding "$MODEL" "$CONTENT"
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
