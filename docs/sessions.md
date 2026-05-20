# Sessions

See [README — Sessions](https://github.com/Ricardo-M-L/anthrogo#sessions).

anthrogo persists every conversation in a SQLite-backed session store, with full-text search and replay support.

## Session commands

| Command | Description |
|---|---|
| `/sessions list` | List recent sessions |
| `/sessions show <id>` | Show full transcript of a session |
| `/sessions replay <id>` | Re-run a session interactively |
| `/sessions search <query>` | Full-text search across all sessions |
| `/sessions export <id>` | Export session to JSON/Markdown |
| `/sessions delete <id>` | Delete a session |
| `/sessions stats` | Aggregate usage statistics |
| `/sessions diff <id1> <id2>` | Diff two sessions |
| `/sessions reindex` | Rebuild the FTS index |

(Full sessions reference migrating from README — M11.4 follow-up.)
