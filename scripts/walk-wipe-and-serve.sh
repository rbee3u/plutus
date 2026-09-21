#!/usr/bin/env bash
# Human walkthrough: rebuild the embedded SPA, reset plutus_walk with walkthrough,
# then serve on :8082. Does not touch the local or agent databases.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

config="config/walk.toml"
envfile="config/walk.env"

if [[ ! -f "$config" || ! -f "$envfile" ]]; then
  echo "Missing $config and/or $envfile." >&2
  echo "Copy config/walk-example.toml to $config and config/walk-example.env to $envfile, then set PLUTUS_DATABASE_DSN and a unique PLUTUS_MASTER_KEY." >&2
  echo "Create the MySQL database once: CREATE DATABASE plutus_walk;" >&2
  echo "Do not point this lane at the local plutus_local database." >&2
  exit 1
fi

echo "Resetting plutus_walk with the walkthrough seed, then serving on 127.0.0.1:8082."
echo

make web-build
go run ./cmd/plutus --config "$config" --env-file "$envfile" db reset --profile walkthrough

echo
echo "Seed logins:"
echo "  alice / Alice=123456"
echo "  bob / Bob=123456"
echo "Open http://127.0.0.1:8082 — not :8080 or :8081."
echo "The next run of this script will reset plutus_walk again."
echo

exec go run ./cmd/plutus --config "$config" --env-file "$envfile" serve
