# Skills

A Skill is a Markdown file the model can invoke on demand via the `Skill` tool. Skills live in `~/.anthrogo/skills/` (home, all projects) or `<cwd>/.anthrogo/skills/` (project-local; overrides same-named home skill).

## Directory layout

```
~/.anthrogo/skills/<name>/
├── SKILL.md                # required — frontmatter + instructions
├── scripts/                # optional — model reads via Read tool
└── references/             # optional
```

## SKILL.md format

```markdown
---
name: git-flow
description: Use when starting a new feature branch off main.
---

# git-flow

When the user asks to start a new branch:
1. Run `git checkout main && git pull`
2. Run `git checkout -b feature/<name>`
3. Confirm the branch was created with `git branch --show-current`
```

`name` must match the directory name. `description` is required — it is shown to the model in the system prompt so it can decide when to invoke the skill.

## How skills work

anthrogo lists every loaded skill (name + description) in the system prompt at startup. The model picks one, calls the `Skill` tool with `{"skill": "git-flow"}`, and receives the full SKILL.md back as a tool result. It then follows the instructions using Read / Bash / Write / etc. — all still gated by the existing permission rules.

## Slash commands

```
/skills                # list all loaded skills
/skills show <name>    # print one skill's SKILL.md
/skills reload         # re-scan ~/.anthrogo/skills/ and <cwd>/.anthrogo/skills/
```

## Precedence

Project-level `<cwd>/.anthrogo/skills/<name>/SKILL.md` overrides a same-named home skill. The override is exact-match on the directory name.

## Trust

The body of a SKILL.md becomes part of the prompt sent to the model when invoked. A malicious skill can instruct the model to leak data, exfiltrate files, or trigger side effects — though every action still flows through anthrogo's tool permission gate. Only install skills from sources you trust.

## Example: code-review skill

```markdown
---
name: code-review
description: Use when asked to review code changes or a pull request.
---

# code-review

1. Run `git diff HEAD~1` to see recent changes.
2. Check for: unhandled errors, missing tests, hardcoded secrets, logic bugs.
3. For each issue found: cite the file and line number, explain the problem, suggest a fix.
4. Output a summary section at the end.
```
