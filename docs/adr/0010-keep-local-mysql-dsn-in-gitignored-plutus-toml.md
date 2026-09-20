---
status: accepted
---

# Keep the local MySQL DSN in gitignored `plutus.toml`

Local development needs a durable MySQL connection string without exporting environment variables in every shell. Put the DSN in the gitignored working-directory `plutus.toml`, allow `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` to override it, and keep the Connection encryption master key out of TOML. The DSN still must never appear in logs, audit events, or committed example secrets that are real credentials.
