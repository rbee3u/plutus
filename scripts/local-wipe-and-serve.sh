#!/usr/bin/env bash
# dev (local): rebuild the embedded SPA, reset plutus_local with the walkthrough seed, then serve on :8080.
# Use local-serve.sh to keep the existing data. Does not touch plutus_agent.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

config="config/local.toml"
envfile="config/local.env"

if [[ ! -f "$config" || ! -f "$envfile" ]]; then
  echo "Missing $config and/or $envfile." >&2
  echo "Copy config/local-example.toml to $config and config/local-example.env to $envfile, then set PLUTUS_DSN and a unique PLUTUS_MASTER_KEY." >&2
  echo "Create the MySQL database once: CREATE DATABASE plutus_local;" >&2
  exit 1
fi

echo "Resetting plutus_local with the walkthrough seed, then serving on 0.0.0.0:8080."
echo

make web-build
go run ./cmd/plutus --config "$config" --env-file "$envfile" db reset --profile walkthrough

echo
echo "Seed logins:"
echo "  alice / Alice=123456"
echo "  bob / Bob=123456"
echo "This machine: http://127.0.0.1:8080"
lan_ip="$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || true)"
if [[ -n "$lan_ip" ]]; then
  echo "LAN: http://${lan_ip}:8080"
fi
echo "The next run of this script will reset plutus_local again; local-serve.sh keeps the data."
echo

exec go run ./cmd/plutus --config "$config" --env-file "$envfile" serve
