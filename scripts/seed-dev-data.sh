#!/usr/bin/env bash
set -euo pipefail

distribution_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
backend_dir="${MAGPIE_BACKEND_REPO:-$distribution_dir/../magpie-backend}"

if [[ ! -f "$backend_dir/go.mod" ]]; then
  echo "Clone magpie-backend next to magpie or set MAGPIE_BACKEND_REPO." >&2
  exit 1
fi

cd -- "$backend_dir"
exec go run ./cmd/seed-dev-data "$@"
