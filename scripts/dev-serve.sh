#!/usr/bin/env bash
# Deprecated alias for scripts/local-serve.sh.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
exec "$root/scripts/local-serve.sh"
