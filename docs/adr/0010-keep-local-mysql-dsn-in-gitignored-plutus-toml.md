---
status: accepted
---

# Keep the local MySQL DSN in gitignored files

Local development needs a durable MySQL connection string without exporting environment variables in every shell. Put the DSN in a gitignored env file under `config/` (`local.env`, `agent.env`, or `walk.env`), allow `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` to override it, and keep the Connection encryption master key out of TOML. A gitignored TOML file may still hold a DSN for convenience. The DSN still must never appear in logs, audit events, or committed example secrets that are real credentials.

This record originally placed the DSN in a working-directory `plutus.toml`. Lane files later moved into `config/` so local, agent, and walkthrough settings sit together.
