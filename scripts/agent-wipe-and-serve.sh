#!/usr/bin/env bash
# dev (agent): rebuild the embedded SPA, reset plutus_agent with the walkthrough seed, then serve on :8081.
# Use agent-serve.sh to keep the existing data. Does not touch plutus_local.
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

echo "Resetting plutus_agent with the walkthrough seed, then serving on 127.0.0.1:8081."
echo

make web-build
go run ./cmd/plutus --config "$config" --env-file "$envfile" db reset --profile walkthrough

echo
echo "Seed logins:"
echo "  alice / Alice=123456"
echo "  bob / Bob=123456"
echo "Open http://127.0.0.1:8081 (not :8080)."
echo "The next run of this script will reset plutus_agent again; agent-serve.sh keeps the data."
echo

exec go run ./cmd/plutus --config "$config" --env-file "$envfile" serve
