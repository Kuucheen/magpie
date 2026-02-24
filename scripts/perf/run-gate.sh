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

READ_DURATION="${PERF_READ_DURATION:-30m}"
WRITE_DURATION="${PERF_WRITE_DURATION:-30m}"
SOAK_DURATION="${PERF_SOAK_DURATION:-0}"

START_SNAPSHOT="${ARTIFACT_DIR}/snapshot.start.env"
END_SNAPSHOT="${ARTIFACT_DIR}/snapshot.end.env"

echo "Capturing start snapshot..."
"${SCRIPT_DIR}/capture-snapshot.sh" >"${START_SNAPSHOT}"

echo "Running read-path suite (${READ_DURATION})..."
k6 run -e PERF_DURATION="${READ_DURATION}" "${K6_DIR}/read-path.js"

echo "Running write-path suite (${WRITE_DURATION})..."
k6 run -e PERF_DURATION="${WRITE_DURATION}" "${K6_DIR}/write-path.js"

if [ "${SOAK_DURATION}" != "0" ]; then
  echo "Running mixed-soak suite (${SOAK_DURATION})..."
  k6 run -e PERF_SOAK_DURATION="${SOAK_DURATION}" "${K6_DIR}/mixed-soak.js"
else
  echo "Skipping mixed-soak suite (set PERF_SOAK_DURATION to run it)."
fi

echo "Capturing end snapshot..."
"${SCRIPT_DIR}/capture-snapshot.sh" >"${END_SNAPSHOT}"

echo "Checking queue/db growth budgets..."
"${SCRIPT_DIR}/assert-snapshot-delta.sh" "${START_SNAPSHOT}" "${END_SNAPSHOT}"

echo "Checking dropped-statistics budget..."
"${SCRIPT_DIR}/check-drop-budget.sh"

echo "Performance gate PASS."
echo "Artifacts:"
echo "  ${START_SNAPSHOT}"
echo "  ${END_SNAPSHOT}"
