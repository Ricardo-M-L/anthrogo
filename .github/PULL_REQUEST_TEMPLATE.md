<!--
Thanks for opening a PR! A few quick checks first.

For SECURITY fixes please open a private report per SECURITY.md instead of a public PR.
-->

## Summary

<!-- One paragraph. What does this PR do? What problem does it solve? -->

## Type of change

- [ ] Bug fix (non-breaking)
- [ ] New feature (non-breaking)
- [ ] Breaking change (existing config / API / CLI surface changes)
- [ ] Documentation
- [ ] CI / build / release infrastructure
- [ ] Test-only

## How was this tested?

<!--
- Did you add a regression test? Where?
- Did you run `go test ./...`? `go test -race ./...`?
- Did you run `anthrogo doctor` and confirm no new failures?
- If touching the TUI / web UI, did you run it locally and verify visually?
-->

## Checklist

- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes on the packages I touched
- [ ] `go vet ./...` is clean
- [ ] New code has tests; touched code has tests covering the changed behavior
- [ ] CHANGELOG.md updated under a new version heading (if user-visible)
- [ ] Docs updated (if user-visible config / command / flag changes)
- [ ] No new external dependencies (or a brief justification in this PR description)
- [ ] No `Co-Authored-By` trailers in commit messages

## Related issues

<!-- Closes #123, refs #456 -->
