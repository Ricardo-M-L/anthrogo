# 03 — Custom skill

**What you'll learn:** write a `SKILL.md`, understand its frontmatter, and
invoke the skill via natural language or the Task tool.

**Time:** ~10 minutes.

---

## What is a skill?

A skill is a Markdown file (`SKILL.md`) stored in a subdirectory of your
`skillsDir`. It contains:

- **YAML frontmatter** — name, description, trigger phrases, version.
- **Body** — instructions the model follows when the skill is invoked.

When a user message matches one of the `triggers`, anthrogo loads the skill
body into the model's context as an additional system instruction.

---

## Prerequisites

| Item | How to get it |
|------|---------------|
| `anthrogo` binary | see [01-basic-chat](../01-basic-chat/) |
| `ANTHROPIC_API_KEY` | see [01-basic-chat](../01-basic-chat/) |

---

## Directory layout

```
03-custom-skill/
├── README.md          ← this file
├── settings.yaml
└── skills/
    └── git-flow/
        └── SKILL.md   ← the sample skill
```

---

## Step 1 — Start anthrogo

```bash
cd examples/03-custom-skill
ANTHROGO_HOME=$(pwd) anthrogo
```

---

## Step 2 — Trigger the skill naturally

Type a phrase that matches one of the `triggers` in the frontmatter:

```
I want to start a feature
```

anthrogo loads `skills/git-flow/SKILL.md` and the model begins guiding you
through the workflow.

---

## Step 3 — Invoke via Task tool (programmatic)

From another skill or a tool call you can invoke a skill by name:

```json
{
  "tool": "Task",
  "input": {
    "skill": "git-flow",
    "args": "branch name: feat/my-awesome-feature"
  }
}
```

The model will execute the `git-flow` skill with the provided arguments.

---

## Step 4 — Write your own skill

1. Create a new subdirectory under `skills/`:

   ```bash
   mkdir -p skills/my-skill
   ```

2. Create `skills/my-skill/SKILL.md` with the following template:

   ```markdown
   ---
   name: my-skill
   description: One-line description of what this skill does.
   triggers:
     - "phrase that invokes the skill"
   version: "1.0.0"
   ---

   # my-skill

   (Instructions for the model go here.)
   ```

3. Restart anthrogo (or `/reload` if that command is available) and test a
   trigger phrase.

---

## SKILL.md frontmatter reference

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique identifier used by the Task tool |
| `description` | yes | Short description shown in `/skills list` |
| `triggers` | no | Natural-language phrases that auto-invoke the skill |
| `version` | no | Semver string for your own tracking |

---

## Next steps

- **Bundle skills into a plugin** → [04-plugin-bundle](../04-plugin-bundle/)
- **Skills reference** → `docs/skills.md`
