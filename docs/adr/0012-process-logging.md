---
status: accepted
---

# Unified process logging and sink ownership

Use `slog` with framework-independent ownership of application, HTTP access, and
SQL file sinks and their lifetime. HTTP integration stays at transport; SQL
interception and diagnostic rendering stay at MySQL. One TOML logging section
supplies configuration with code defaults and no environment overrides.
Application records also go to stderr; access and SQL normally go only to files.

SQL logging is enabled by default with substituted parameter values and
transaction boundaries, without redaction. Result rows, connection
configuration, CLI passwords, master keys, and runtime Platform payloads are not
inputs. Requests and jobs correlate through context; panic diagnostics include
exception and stack information without request dumps.

Writes are synchronous with size-based rotation and one process per directory.
Startup fails if sinks cannot open; active write and rotation failures fall back
to stderr. Lumberjack is the rotation dependency, with asynchronous backup
compression and retention.

Separate sinks support inspection while keeping one application logging API.
Full-value SQL favors diagnosis over confidential parameter handling and disk
use; synchronous writes add latency. Diagnostic SQL is not the driver's wire
representation, and query timing ends before result consumption. Uncertain
rendering falls back to template, arguments, and a rendering error.

Directory ownership is a deployment contract rather than an enforced lock.
Retention is eventual, not a disk quota; asynchronous cleanup errors are not
returned to the application and close does not await cleanup. Oversized records
fall back to stderr. [Architecture](../architecture.md#operations-and-limits),
[configuration](../development.md#logging), and
[deployment](../deployment.md#logs) own implementation and operation.
