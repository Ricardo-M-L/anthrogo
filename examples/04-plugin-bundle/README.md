# 04 — Plugin bundle

**What you'll learn:** the full plugin layout — manifest (`plugin.yaml`),
a slash command, a skill, and a post-message hook — then install and inspect
the plugin at runtime.

**Time:** ~15 minutes.

---

## What is a plugin?

A plugin is a directory that contains:

| File | Purpose |
|------|---------|
| `plugin.yaml` | Manifest: name, version, list of contributed files |
| `commands/*.yaml` | New slash commands |
| `skills/*/SKILL.md` | Skills (like standalone skills, but scoped to the plugin) |
| `hooks/*.sh` | Shell scripts run at lifecycle events |

Plugins let you package related functionality and share it as a single unit.

---

## Prerequisites

| Item | How to get it |
|------|---------------|
| `anthrogo` binary | see [01-basic-chat](../01-basic-chat/) |
| `ANTHROPIC_API_KEY` | see [01-basic-chat](../01-basic-chat/) |

---

## Directory layout

```
04-plugin-bundle/
├── README.md
├── settings.yaml
└── plugins/
    └── sample-plugin/
        ├── plugin.yaml              ← manifest
        ├── commands/
        │   └── greet.yaml           ← /greet slash command
        ├── skills/
        │   └── welcome/
        │       └── SKILL.md         ← welcome skill
        └── hooks/
            └── log.sh               ← post-message hook
```

---

## Step 1 — Start anthrogo

```bash
cd examples/04-plugin-bundle
ANTHROGO_HOME=$(pwd) anthrogo
```

On startup anthrogo scans `plugins/` and loads `sample-plugin`.

---

## Step 2 — List installed plugins

```
/plugin list
```

Expected output:

```
Installed plugins (1)

  sample-plugin  v1.0.0  — A demonstration plugin ...
    Commands : /greet
    Skills   : welcome
    Hooks    : log.sh (post-message)
```

---

## Step 3 — Inspect the plugin

```
/plugin info sample-plugin
```

Expected output includes the manifest fields from `plugin.yaml`.

---

## Step 4 — Run the custom command

```
/greet Alice
```

Expected response (abbreviated):

```
Hello Alice! Welcome to anthrogo — you're using the sample-plugin. ...
```

---

## Step 5 — Trigger the bundled skill

```
help me get started
```

The `welcome` skill fires and the model gives you an orientation.

---

## Step 6 — Check the hook log

After a few assistant messages the hook will have written to its log:

```bash
cat /tmp/anthrogo-plugin-demo.log
```

Expected output:

```
[2026-05-21 12:34:56] session=abc123 words=42
[2026-05-21 12:34:58] session=abc123 words=37
```

---

## Step 7 — Install a plugin from a URL (optional)

You can also install plugins from a git repo or tarball URL:

```
/plugin install https://github.com/example/my-anthrogo-plugin
```

---

## Authoring checklist

When writing your own plugin:

- [ ] `plugin.yaml` has unique `name` and valid `version`
- [ ] All paths in `commands:`, `skills:`, `hooks:` are relative to the plugin directory
- [ ] Hook scripts are executable (`chmod +x`)
- [ ] Test with `/plugin list` and `/plugin info <name>`

---

## Next steps

- **KAIROS worker setup** → [05-kairos-worker](../05-kairos-worker/)
- **Plugins reference** → `docs/plugins.md`
- **Hooks reference** → `docs/hooks.md`
