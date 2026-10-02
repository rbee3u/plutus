#!/usr/bin/env bash
# dev (agent): rebuild the SPA and serve the existing plutus_agent data on :8081 without resetting it.
# Use agent-wipe-and-serve.sh for a fresh seeded database. Does not touch plutus_local.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

config="config/agent.toml"
envfile="config/agent.env"

if [[ ! -f "$config" || ! -f "$envfile" ]]; then
  echo "Missing $config and/or $envfile." >&2
  echo "Copy config/agent-example.toml to $config and config/agent-example.env to $envfile, then set PLUTUS_DSN and a unique PLUTUS_MASTER_KEY." >&2
  echo "Create the MySQL database once: CREATE DATABASE plutus_agent;" >&2
  echo "Do not point this environment at plutus_local." >&2
  exit 1
fi

echo "Serving the dev (agent) environment on :8081 using $config and $envfile."
echo "This script does not reset the database."
echo "Open http://127.0.0.1:8081 (not :8080)."
echo

exec make run ARGS="--config $config --env-file $envfile serve"
