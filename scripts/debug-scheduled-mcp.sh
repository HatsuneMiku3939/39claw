#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/debug-scheduled-mcp.sh <endpoint> init
  scripts/debug-scheduled-mcp.sh <endpoint> list-tools
  scripts/debug-scheduled-mcp.sh <endpoint> list-tasks
  scripts/debug-scheduled-mcp.sh <endpoint> call <tool-name> [arguments-json]

Examples:
  scripts/debug-scheduled-mcp.sh http://127.0.0.1:34725/mcp/scheduled-tasks init
  scripts/debug-scheduled-mcp.sh http://127.0.0.1:34725/mcp/scheduled-tasks list-tools
  scripts/debug-scheduled-mcp.sh http://127.0.0.1:34725/mcp/scheduled-tasks list-tasks
  scripts/debug-scheduled-mcp.sh http://127.0.0.1:34725/mcp/scheduled-tasks call scheduled_tasks_get '{"name":"daily-report"}'

Environment:
  MCP_PROTOCOL_VERSION   Override the initialize protocol version. Default: 2025-11-25
  MCP_CLIENT_NAME        Override the initialize client name. Default: debug-curl
  MCP_CLIENT_VERSION     Override the initialize client version. Default: 0.0.0
  MCP_SKIP_INITIALIZED   Set to true to skip notifications/initialized after initialize.
EOF
}

fail() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

cleanup_tmp_dir() {
  if [[ -n "${DEBUG_SCHEDULED_MCP_TMP_DIR:-}" ]]; then
    rm -rf -- "${DEBUG_SCHEDULED_MCP_TMP_DIR}"
  fi
}

pretty_print_json() {
  if command -v jq >/dev/null 2>&1; then
    jq . "$1"
  else
    cat "$1"
  fi
}

extract_session_id() {
  awk '
    BEGIN { IGNORECASE = 1 }
    /^Mcp-Session-Id:/ {
      value = $0
      sub(/^[^:]*:[[:space:]]*/, "", value)
      sub(/\r$/, "", value)
      print value
      exit
    }
  ' "$1"
}

post_json() {
  local endpoint="$1"
  local session_id="${2:-}"
  local payload="$3"
  local headers_file="$4"
  local body_file="$5"

  local curl_args=(
    -sS
    -D "$headers_file"
    -o "$body_file"
    -X POST
    "$endpoint"
    -H "Content-Type: application/json"
    -H "Accept: application/json, text/event-stream"
    --data "$payload"
  )

  if [[ -n "$session_id" ]]; then
    curl_args+=(-H "Mcp-Session-Id: $session_id")
  fi

  curl "${curl_args[@]}"
}

send_initialized_notification() {
  local endpoint="$1"
  local session_id="$2"
  local headers_file="$3"
  local body_file="$4"
  local payload='{"jsonrpc":"2.0","method":"notifications/initialized"}'

  post_json "$endpoint" "$session_id" "$payload" "$headers_file" "$body_file"
}

main() {
  require_command curl

  local endpoint="${1:-}"
  local action="${2:-}"

  [[ -n "$endpoint" ]] || { usage; exit 1; }
  [[ -n "$action" ]] || { usage; exit 1; }

  local protocol_version="${MCP_PROTOCOL_VERSION:-2025-11-25}"
  local client_name="${MCP_CLIENT_NAME:-debug-curl}"
  local client_version="${MCP_CLIENT_VERSION:-0.0.0}"

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  DEBUG_SCHEDULED_MCP_TMP_DIR="$tmp_dir"
  trap cleanup_tmp_dir EXIT

  local init_headers="$tmp_dir/init.headers"
  local init_body="$tmp_dir/init.body"
  local init_payload
  init_payload="$(cat <<EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"$protocol_version","clientInfo":{"name":"$client_name","version":"$client_version"},"capabilities":{}}}
EOF
)"

  post_json "$endpoint" "" "$init_payload" "$init_headers" "$init_body"

  local session_id
  session_id="$(extract_session_id "$init_headers")"
  [[ -n "$session_id" ]] || fail "initialize response did not include Mcp-Session-Id"

  if [[ "${MCP_SKIP_INITIALIZED:-false}" != "true" ]]; then
    send_initialized_notification "$endpoint" "$session_id" "$tmp_dir/initialized.headers" "$tmp_dir/initialized.body"
  fi

  if [[ "$action" == "init" ]]; then
    printf 'session_id=%s\n' "$session_id"
    pretty_print_json "$init_body"
    exit 0
  fi

  local request_id=2
  local payload
  case "$action" in
    list-tools)
      payload='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
      ;;
    list-tasks)
      payload='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"scheduled_tasks_list","arguments":{}}}'
      ;;
    call)
      local tool_name="${3:-}"
      local arguments_json="${4:-{}}"
      [[ -n "$tool_name" ]] || fail "call requires a tool name"
      payload="$(cat <<EOF
{"jsonrpc":"2.0","id":$request_id,"method":"tools/call","params":{"name":"$tool_name","arguments":$arguments_json}}
EOF
)"
      ;;
    *)
      usage
      exit 1
      ;;
  esac

  local response_headers="$tmp_dir/response.headers"
  local response_body="$tmp_dir/response.body"

  post_json "$endpoint" "$session_id" "$payload" "$response_headers" "$response_body"

  printf 'session_id=%s\n' "$session_id"
  pretty_print_json "$response_body"
}

main "$@"
