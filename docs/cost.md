# Cost tracking

See [README — Cost](https://github.com/Ricardo-M-L/anthrogo#cost).

anthrogo tracks token usage and estimated cost for every session using built-in pricing tables.

## Usage

```
/cost
```

Displays a breakdown of input/output tokens and estimated USD cost for the current session.

## Budget caps

Set a per-session budget cap in `settings.yaml`:

```yaml
cost:
  budget_usd: 1.00   # hard stop when session cost exceeds $1.00
```

When the cap is reached, anthrogo will refuse to send further requests and prompt you to start a new session or raise the cap.

(Full cost reference migrating from README — M11.4 follow-up.)
