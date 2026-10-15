#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
K6_DIR="${SCRIPT_DIR}/k6"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 is required. Install from https://k6.io/docs/get-started/installation/" >&2
  exit 1
fi

ARTIFACT_DIR="${PERF_ARTIFACT_DIR:-${REPO_ROOT}/artifacts/perf}"
mkdir -p "${ARTIFACT_DIR}"

GATE_MODE="${PERF_GATE_MODE:-release}"
case "${GATE_MODE}" in
  release)
    READ_DURATION="${PERF_READ_DURATION:-30m}"
    WRITE_DURATION="${PERF_WRITE_DURATION:-30m}"
    SOAK_DURATION="${PERF_SOAK_DURATION:-24h}"
    if [[ "${SOAK_DURATION}" =~ ^([0-9]+)(s|m|h)$ ]]; then
      soak_seconds="${BASH_REMATCH[1]}"
      case "${BASH_REMATCH[2]}" in m) soak_seconds=$((soak_seconds * 60));; h) soak_seconds=$((soak_seconds * 3600));; esac
    else
      echo "Release soak duration must be an integer followed by s, m or h." >&2
      exit 1
    fi
    if [ "${soak_seconds}" -lt 86400 ] || [ "${PERF_DROP_BUDGET_SOURCE:-auto}" = "skip" ]; then
      echo "Release validation requires at least 24h of mixed soak and a drop-budget log source. Use PERF_GATE_MODE=smoke for shorter checks." >&2
      exit 1
    fi
    ;;
  smoke)
    READ_DURATION="${PERF_READ_DURATION:-1m}"
    WRITE_DURATION="${PERF_WRITE_DURATION:-1m}"
    SOAK_DURATION="${PERF_SOAK_DURATION:-2m}"
    ;;
  *) echo "PERF_GATE_MODE must be release or smoke." >&2; exit 1;;
esac
export PERF_GATE_MODE="${GATE_MODE}"
export PERF_DROP_LOG_SINCE="${PERF_DROP_LOG_SINCE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

START_SNAPSHOT="${ARTIFACT_DIR}/snapshot.start.env"
END_SNAPSHOT="${ARTIFACT_DIR}/snapshot.end.env"

echo "Capturing start snapshot..."
"${SCRIPT_DIR}/capture-snapshot.sh" >"${START_SNAPSHOT}"

echo "Running read-path suite (${READ_DURATION})..."
k6 run --summary-export="${ARTIFACT_DIR}/read.summary.json" -e PERF_DURATION="${READ_DURATION}" "${K6_DIR}/read-path.js"

echo "Running write-path suite (${WRITE_DURATION})..."
k6 run --summary-export="${ARTIFACT_DIR}/write.summary.json" -e PERF_DURATION="${WRITE_DURATION}" "${K6_DIR}/write-path.js"

if [ "${SOAK_DURATION}" != "0" ]; then
  echo "Running mixed-soak suite (${SOAK_DURATION})..."
  k6 run --summary-export="${ARTIFACT_DIR}/soak.summary.json" -e PERF_SOAK_DURATION="${SOAK_DURATION}" "${K6_DIR}/mixed-soak.js"
else
  echo "Skipping mixed-soak suite (set PERF_SOAK_DURATION to run it)."
fi

echo "Capturing end snapshot..."
"${SCRIPT_DIR}/capture-snapshot.sh" >"${END_SNAPSHOT}"

echo "Checking queue/db growth budgets..."
"${SCRIPT_DIR}/assert-snapshot-delta.sh" "${START_SNAPSHOT}" "${END_SNAPSHOT}"

echo "Checking dropped-statistics budget..."
"${SCRIPT_DIR}/check-drop-budget.sh"

echo "Performance ${GATE_MODE} validation PASS."
if [ "${GATE_MODE}" = "smoke" ]; then
  echo "A smoke run does not establish production capacity or durability."
fi
echo "Artifacts:"
echo "  ${START_SNAPSHOT}"
echo "  ${END_SNAPSHOT}"
