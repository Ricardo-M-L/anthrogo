#!/usr/bin/env bash
# hooks/log.sh — post-message hook for the sample plugin.
#
# anthrogo runs this script after every assistant message.
# Environment variables available:
#   ANTHROGO_SESSION_ID   — unique ID for the current session
#   ANTHROGO_MESSAGE_ROLE — "user" or "assistant"
#   ANTHROGO_MESSAGE_TEXT — the full text of the message (may be large)
#   ANTHROGO_TIMESTAMP    — Unix timestamp of the message
#
# This example appends a one-line log entry to /tmp/anthrogo-plugin-demo.log.
# In a real plugin you might post to a webhook, update a database, etc.

set -euo pipefail

LOG_FILE="/tmp/anthrogo-plugin-demo.log"

# Only log assistant messages (skip user turns).
if [[ "${ANTHROGO_MESSAGE_ROLE:-}" != "assistant" ]]; then
  exit 0
fi

TIMESTAMP="${ANTHROGO_TIMESTAMP:-$(date +%s)}"
SESSION="${ANTHROGO_SESSION_ID:-unknown}"
WORDS=$(echo "${ANTHROGO_MESSAGE_TEXT:-}" | wc -w | tr -d ' ')

echo "[$(date -r "$TIMESTAMP" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || date '+%Y-%m-%d %H:%M:%S')] session=$SESSION words=$WORDS" >> "$LOG_FILE"
