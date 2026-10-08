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

Use one TOML `[logging]` section with code defaults and no environment
overrides. Single-line application records also go to stderr; access and SQL
normally go only to files.

SQL logging is enabled by default, including substituted parameter values and
transaction boundaries without redaction. Result rows, connection configuration,
CLI passwords, master keys, and runtime Platform payloads are not logging
inputs.

Use synchronous writes and size-based rotation. One process owns each directory.
Startup fails if sinks cannot open; active write failures fall back to stderr.
Lumberjack handles backup compression and retention asynchronously, including
maintenance deferred by short-lived commands.

Correlate requests and jobs through context. Access records include rejections
and recovered requests; panic diagnostics contain exceptions and stacks without
request dumps.

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
