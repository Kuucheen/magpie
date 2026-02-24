#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${MAGPIE_COMPOSE_FILE:-${REPO_ROOT}/docker-compose.yml}"
ENV_FILE="${MAGPIE_ENV_FILE:-${REPO_ROOT}/.env}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 1
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD=(docker-compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
else
  echo "docker compose or docker-compose is required" >&2
  exit 1
fi

ensure_service_running() {
  local service="$1"
  local cid
  cid="$("${COMPOSE_CMD[@]}" ps -q "${service}" | tr -d '\r' | head -n1 || true)"

  if [ -z "${cid}" ]; then
    echo "service \"${service}\" is not running for compose file: ${COMPOSE_FILE}" >&2
    echo "env file: ${ENV_FILE}" >&2
    echo "If your stack is in another folder, rerun with:" >&2
    echo "  MAGPIE_COMPOSE_FILE=/path/to/docker-compose.yml MAGPIE_ENV_FILE=/path/to/.env ./run-gate.sh" >&2
    exit 1
  fi

  if [ "$(docker inspect -f '{{.State.Running}}' "${cid}" 2>/dev/null || echo false)" != "true" ]; then
    echo "service \"${service}\" exists but is not running (container: ${cid})" >&2
    echo "Start the stack first, then rerun performance gate." >&2
    exit 1
  fi
}

ensure_service_running redis
ensure_service_running postgres

proxy_queue_depth="$("${COMPOSE_CMD[@]}" exec -T redis redis-cli --raw ZCARD proxy_queue | tr -d '\r')"
scrapesite_queue_depth="$("${COMPOSE_CMD[@]}" exec -T redis redis-cli --raw ZCARD scrapesite_queue | tr -d '\r')"

db_snapshot="$("${COMPOSE_CMD[@]}" exec -T postgres sh -lc '
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" <<'"'"'SQL'"'"'
SELECT
  (SELECT COUNT(*) FROM proxies),
  (SELECT COUNT(*) FROM proxy_statistics),
  (SELECT COUNT(*) FROM proxy_statistics WHERE response_body IS NOT NULL AND response_body <> '"'"''"'"'),
  (SELECT pg_total_relation_size('"'"'proxy_statistics'"'"')),
  (SELECT pg_database_size(current_database()));
SQL
')"

IFS="|" read -r proxy_count proxy_statistics_count proxy_statistics_response_rows proxy_statistics_total_bytes database_total_bytes <<<"${db_snapshot}"

timestamp_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
timestamp_epoch="$(date -u +%s)"

cat <<EOF
timestamp_utc=${timestamp_utc}
timestamp_epoch=${timestamp_epoch}
proxy_queue_depth=${proxy_queue_depth}
scrapesite_queue_depth=${scrapesite_queue_depth}
proxy_count=${proxy_count}
proxy_statistics_count=${proxy_statistics_count}
proxy_statistics_response_rows=${proxy_statistics_response_rows}
proxy_statistics_total_bytes=${proxy_statistics_total_bytes}
database_total_bytes=${database_total_bytes}
EOF
