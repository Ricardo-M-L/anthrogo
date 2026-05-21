#!/usr/bin/env bash
set -euo pipefail
# Generate Markdown API docs for each pkg/ + internal/ package using go doc.
# Output: docs/api/<pkg-path>.md

cd "$(dirname "$0")/.."
mkdir -p docs/api

packages=$(go list ./pkg/... ./internal/...)

for pkg in $packages; do
    rel="${pkg#github.com/ricardo/anthrogo/}"
    out="docs/api/${rel//\//_}.md"
    {
        echo "# \`$pkg\`"
        echo ""
        echo '```go'
        go doc -all "$pkg" 2>/dev/null || true
        echo '```'
    } > "$out"
done

# Index
{
    echo "# API reference"
    echo ""
    echo "Auto-generated from godoc. See [pkg.go.dev](https://pkg.go.dev/github.com/ricardo/anthrogo) for the rendered version."
    echo ""
    echo "| Package | Description |"
    echo "|---|---|"
    for pkg in $packages; do
        rel="${pkg#github.com/ricardo/anthrogo/}"
        out="${rel//\//_}.md"
        first_line=$(go doc "$pkg" 2>/dev/null | sed -n '2p' | head -c 80)
        echo "| [$rel]($out) | $first_line |"
    done
} > docs/api/index.md
