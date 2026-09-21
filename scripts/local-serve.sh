#!/usr/bin/env bash
# Local development serve on :8080. Does not wipe the database or rebuild the SPA.
# Use Vite (`npm run dev` in web/) against this process. Agent self-check and
# human walkthrough use the other scripts.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

config="config/local.toml"
envfile="config/local.env"

if [[ ! -f "$config" || ! -f "$envfile" ]]; then
  echo "Missing $config and/or $envfile." >&2
  echo "Copy config/local-example.toml to $config and config/local-example.env to $envfile, then set PLUTUS_DATABASE_DSN and a unique PLUTUS_MASTER_KEY." >&2
  echo "Create the MySQL database once: CREATE DATABASE plutus_local;" >&2
  exit 1
fi

echo "Starting local serve on 0.0.0.0:8080 using $config and $envfile."
echo "This script does not reset the database."
echo "This machine: http://127.0.0.1:8080"
lan_ip="$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || true)"
if [[ -n "$lan_ip" ]]; then
  echo "LAN: http://${lan_ip}:8080"
fi
echo

exec go run ./cmd/plutus --config "$config" --env-file "$envfile" serve
