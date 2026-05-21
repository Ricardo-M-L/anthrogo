---
name: welcome
description: >
  Greets a new user and gives them a brief orientation to anthrogo.
  Triggered when the user says "help me get started" or similar.
triggers:
  - "help me get started"
  - "what can you do"
  - "welcome"
version: "1.0.0"
---

# welcome skill

You are welcoming a new user to anthrogo. Give them a concise, friendly
orientation covering the following points:

1. **What anthrogo is** — a terminal-based AI assistant built on Claude.

2. **Key slash commands to know:**
   - `/help` — list all available commands
   - `/usage` — show token usage for this session
   - `/cost` — show estimated cost for this session
   - `/compact` — summarise the conversation to save context
   - `/exit` — quit

3. **Installed plugins** — mention that the `sample-plugin` is loaded and they
   can try `/greet <name>` to see it in action.

4. **Getting more help** — point them to the full documentation at
   https://Ricardo-M-L.github.io/anthrogo/

Keep the response friendly and under 150 words.
