#!/usr/bin/env bash
# Temporary early-development helper: wipe the local plutus database, migrate,
# create a scratch user, then start serve. Delete this when a wipe-on-start
# workflow is no longer useful. It is not a substitute for Phase 2 db reset.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

username="admin"
password="Admin=123456"

echo "Temporary wipe-and-serve helper (safe to delete later)."
echo "This drops every table in the configured database named plutus."
echo

go run ./scripts/devwipe
go run ./cmd/plutus migrate up
go run ./cmd/plutus user create --username "$username" --password "$password"

echo
echo "Scratch login: ${username} / ${password}"
echo "Starting serve. The next run of this script will wipe the database again."
echo

exec go run ./cmd/plutus serve
