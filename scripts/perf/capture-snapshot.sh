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

# Include legacy queues and all supported shards, even from older shard layouts.
# Keep these bounds aligned with backend maxQueueShards/maxScrapeQueueShards.
# Exactly 194 O(1) cardinality reads, independent of the proxy payload keyspace.
queue_snapshot="$("${COMPOSE_CMD[@]}" exec -T redis sh -ec '
export REDISCLI_AUTH="${REDIS_PASSWORD:-}"
exec redis-cli --raw EVAL "$1" 0
' sh '
local proxy = redis.call("ZCARD", "proxy_queue")
local scrape = redis.call("ZCARD", "scrapesite_queue")
for shard = 0, 127 do
  proxy = proxy + redis.call("ZCARD", "proxy_queue:" .. shard)
end
for shard = 0, 63 do
  scrape = scrape + redis.call("ZCARD", "scrapesite_queue:" .. shard)
end
return {proxy, scrape}
' | tr -d '\r')"
{
  IFS= read -r proxy_queue_depth
  IFS= read -r scrapesite_queue_depth
} <<<"${queue_snapshot}"
if ! [[ "${proxy_queue_depth}" =~ ^[0-9]+$ && "${scrapesite_queue_depth}" =~ ^[0-9]+$ ]]; then
  echo "Redis queue measurement failed: ${queue_snapshot}" >&2
  exit 1
fi

db_snapshot="$("${COMPOSE_CMD[@]}" exec -T postgres sh -lc '
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" <<'"'"'SQL'"'"'
SELECT
  (SELECT COUNT(*) FROM proxies),
  (SELECT COUNT(*) FROM proxy_statistics),
  (SELECT COUNT(*) FROM proxy_statistics WHERE response_body IS NOT NULL AND response_body <> '"'"''"'"'),
  (WITH RECURSIVE relations(oid) AS (
    SELECT '"'"'proxy_statistics'"'"'::regclass::oid
    UNION ALL
    SELECT child.inhrelid FROM pg_inherits child JOIN relations parent ON child.inhparent = parent.oid
  ) SELECT COALESCE(SUM(pg_total_relation_size(oid)), 0) FROM relations),
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
