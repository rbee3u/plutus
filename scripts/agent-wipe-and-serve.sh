#!/usr/bin/env bash
# Agent self-check: rebuild the embedded SPA, reset plutus_agent with walkthrough,
# then serve on :8081. Does not touch the local or walkthrough databases.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

config="config/agent.toml"
envfile="config/agent.env"

if [[ ! -f "$config" || ! -f "$envfile" ]]; then
  echo "Missing $config and/or $envfile." >&2
  echo "Copy config/agent-example.toml to $config and config/agent-example.env to $envfile, then set PLUTUS_DATABASE_DSN and a unique PLUTUS_MASTER_KEY." >&2
  echo "Create the MySQL database once: CREATE DATABASE plutus_agent;" >&2
  echo "Do not point this lane at the local plutus_local database." >&2
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
echo "Open http://127.0.0.1:8081 — not :8080 or :8082."
echo "The next run of this script will reset plutus_agent again."
echo

exec go run ./cmd/plutus --config "$config" --env-file "$envfile" serve
