---
name: git-flow
description: >
  Guides the user through a complete Git feature-branch workflow:
  create branch → commit changes → open pull request.
  Invoked automatically when the user asks to "start a feature" or
  "open a PR" without knowing the exact commands.
triggers:
  - "start a feature"
  - "create a feature branch"
  - "open a PR"
  - "open a pull request"
version: "1.0.0"
---

# git-flow skill

You are helping the user follow a standard Git feature-branch workflow.
Work through the steps below in order, confirming with the user at each step
before running any command.

## Step 1 — Ensure a clean working tree

Run:

```bash
git status
```

If there are uncommitted changes, ask the user whether to stash them or commit
them before proceeding.

## Step 2 — Create a feature branch

Ask the user for a short branch name, e.g. `feat/my-feature`.

Run:

```bash
git checkout -b <branch-name>
```

Confirm the branch was created:

```bash
git branch --show-current
```

## Step 3 — Make changes

Tell the user: "Make your code changes now. When you're ready, tell me and I'll
help you commit them."

## Step 4 — Stage and commit

Once the user is ready, show them which files changed:

```bash
git diff --stat
```

Stage all changes:

```bash
git add -A
```

Commit with a conventional-commit message. Ask the user for a short description
of what changed, then run:

```bash
git commit -m "feat: <description>"
```

## Step 5 — Push the branch

```bash
git push -u origin <branch-name>
```

## Step 6 — Open a pull request

If the `gh` CLI is available, run:

```bash
gh pr create --fill
```

Otherwise, print the URL the user can open:

```
https://github.com/<owner>/<repo>/compare/<branch-name>
```

## Done

Summarise what was done: branch name, commit message, and PR URL.
