---
status: accepted
---

# ADR 0010: Unified process logging

## Decision

Keep `slog` as the application logging interface. A framework-independent
logging component owns separate application, HTTP access, and SQL file sinks,
their common configuration, and their lifetime. Gin integration belongs at the
HTTP boundary; SQL interception and diagnostic rendering belong at the MySQL
boundary. Add only lumberjack as the rotation dependency.

Configure logging through the TOML `[logging]` section, with code defaults for
omitted fields. This accepted choice keeps one configuration entry point for
logging; environment overrides are not supported.

Use single-line text records. Application logs also go to stderr; access and SQL
normally go only to their files. SQL is enabled by default and records all
project statements and transaction boundaries, with bound values substituted for
display. This is an accepted operator choice: SQL text and parameter values are
not redacted. Result rows, database connection configuration, CLI password
arguments, master keys, and runtime Platform payloads are not logging inputs.

Use synchronous writes and size-based rotation, with configurable backup count,
age, and compression. One process owns each directory. Initialization failures
stop startup; active write failures fall back to stderr without stopping work.
Backup compression and retention use lumberjack's asynchronous maintenance;
short commands do not wait for maintenance, which resumes on subsequent runs.

Request and job context carries available correlation fields. Access records
cover accepted, rejected, and recovered requests. Gin framework events and panic
diagnostics enter application logging through boundary adapters, with panic
records containing the exception and stack rather than request dumps.

## Reason and trade-offs

Existing services already use `slog`. Retaining that interface avoids a second
logging API and keeps infrastructure dependencies out of application callers.
Three sinks make request traffic and verbose SQL independently readable, while
stderr retains operational visibility. The small rotation dependency reduces the
amount of file lifecycle code the project must maintain.

Full-value SQL logging favors inspection over reduced disk use and confidential
parameter handling. Synchronous file writes add latency, particularly for SQL.
Diagnostic SQL is not the driver's wire representation; uncertain rendering
falls back to the template, arguments, and a rendering error without changing
execution. Query duration stops at driver return, before result consumption.

Single-process ownership is a deployment contract, not an enforced lock.
Lumberjack maintenance is eventually applied, not a strict disk quota: cleanup
and compression failures are not returned to the application, and active-file
close does not wait for them. Application write and rotation failures remain
visible through stderr. Oversized individual records also fall back to stderr.

[Architecture](../architecture.md), [configuration](../development.md#logging),
and [deployment](../deployment.md#logs) describe the implementation and
operation.
