#!/usr/bin/env bash
cat > /dev/null
cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"injected ctx"}}
JSON
exit 0
