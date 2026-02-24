#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${MAGPIE_COMPOSE_FILE:-${REPO_ROOT}/docker-compose.yml}"
ENV_FILE="${MAGPIE_ENV_FILE:-${REPO_ROOT}/.env}"
DROP_BUDGET="${PERF_PROXY_STAT_DROPS_BUDGET:-0}"
LOG_SINCE="${PERF_DROP_LOG_SINCE:-24h}"
DROP_SOURCE="${PERF_DROP_BUDGET_SOURCE:-auto}"
BACKEND_LOG_FILE="${PERF_BACKEND_LOG_FILE:-}"

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

is_backend_container_running() {
  local cid
  cid="$("${COMPOSE_CMD[@]}" ps -q backend | tr -d '\r' | head -n1 || true)"
  if [ -z "${cid}" ]; then
    return 1
  fi

  if [ "$(docker inspect -f '{{.State.Running}}' "${cid}" 2>/dev/null || echo false)" != "true" ]; then
    return 1
  fi

  return 0
}

read_log_stream() {
  local source="$1"
  case "${source}" in
    compose)
      "${COMPOSE_CMD[@]}" logs --since "${LOG_SINCE}" --no-color backend 2>/dev/null || true
      ;;
    file)
      cat "${BACKEND_LOG_FILE}"
      ;;
    *)
      return 1
      ;;
  esac
}

case "${DROP_SOURCE}" in
  auto)
    if is_backend_container_running; then
      resolved_source="compose"
    elif [ -n "${BACKEND_LOG_FILE}" ] && [ -f "${BACKEND_LOG_FILE}" ]; then
      resolved_source="file"
    else
      echo "WARN: skipping dropped-statistics budget check; no backend log source found." >&2
      echo "To enforce this check with local 'go run', set PERF_BACKEND_LOG_FILE=/path/to/backend.log." >&2
      echo "To enforce against compose backend logs, run backend service in compose or set PERF_DROP_BUDGET_SOURCE=compose." >&2
      exit 0
    fi
    ;;
  compose)
    if ! is_backend_container_running; then
      echo "service \"backend\" is not running for compose file: ${COMPOSE_FILE}" >&2
      echo "env file: ${ENV_FILE}" >&2
      echo "If your stack is in another folder, rerun with:" >&2
      echo "  MAGPIE_COMPOSE_FILE=/path/to/docker-compose.yml MAGPIE_ENV_FILE=/path/to/.env ./run-gate.sh" >&2
      exit 1
    fi
    resolved_source="compose"
    ;;
  file)
    if [ -z "${BACKEND_LOG_FILE}" ] || [ ! -f "${BACKEND_LOG_FILE}" ]; then
      echo "PERF_DROP_BUDGET_SOURCE=file requires PERF_BACKEND_LOG_FILE to point to an existing file." >&2
      exit 1
    fi
    resolved_source="file"
    ;;
  skip)
    echo "Skipping dropped-statistics budget check (PERF_DROP_BUDGET_SOURCE=skip)."
    exit 0
    ;;
  *)
    echo "Invalid PERF_DROP_BUDGET_SOURCE='${DROP_SOURCE}'. Use auto|compose|file|skip." >&2
    exit 1
    ;;
esac

log_output="$(read_log_stream "${resolved_source}")"

max_dropped="0"
while IFS= read -r line; do
  if [[ "${line}" =~ dropped_total[=:[:space:]]*([0-9]+) ]]; then
    current="${BASH_REMATCH[1]}"
    if [ "${current}" -gt "${max_dropped}" ]; then
      max_dropped="${current}"
    fi
  fi
done <<<"${log_output}"

echo "proxy_statistics_dropped_total_max=${max_dropped}"
echo "drop_budget=${DROP_BUDGET}"
echo "log_window=${LOG_SINCE}"
echo "log_source=${resolved_source}"

if [ "${max_dropped}" -gt "${DROP_BUDGET}" ]; then
  echo "FAIL: observed dropped_total=${max_dropped} exceeds budget=${DROP_BUDGET}" >&2
  exit 1
fi

echo "PASS: dropped statistics are within budget."
