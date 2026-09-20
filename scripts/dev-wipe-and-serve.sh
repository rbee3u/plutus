#!/usr/bin/env bash
# Development helper: reset the allowed local database with the minimal
# source-fact seed, then start serve. Requires [database].allow_reset = true.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

echo "Resetting the configured development database with the minimal seed."
echo "Requires [database].allow_reset = true in plutus.toml and a database name matching plutus or plutus_<suffix>."
echo

go run ./cmd/plutus db reset --profile minimal

echo
echo "Seed logins:"
echo "  alice / Alice=123456"
echo "  bob / Bob=123456"
echo "Starting serve. The worker activates seeded Portfolio projections."
echo "The next run of this script will reset the database again."
echo

exec go run ./cmd/plutus serve
