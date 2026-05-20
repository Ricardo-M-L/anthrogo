#!/usr/bin/env bash
cat > /dev/null
cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"PreToolUse","modifiedInput":{"command":"ls -al"}}}
JSON
exit 0
