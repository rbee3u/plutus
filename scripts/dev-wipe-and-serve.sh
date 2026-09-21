#!/usr/bin/env bash
# Deprecated alias: this used to wipe the local database. It now starts the
# isolated human walkthrough lane on :8082. There is no local-wipe script;
# local development uses scripts/local-serve.sh without reset.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"

echo "scripts/dev-wipe-and-serve.sh now starts the human walkthrough lane." >&2
echo "Local development without wipe: scripts/local-serve.sh (:8080)." >&2
echo "Agent self-check: scripts/agent-wipe-and-serve.sh (:8081)." >&2
echo "Human walkthrough: scripts/walk-wipe-and-serve.sh (:8082)." >&2
echo >&2

exec "$root/scripts/walk-wipe-and-serve.sh"
