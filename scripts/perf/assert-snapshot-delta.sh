#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <start_snapshot.env> <end_snapshot.env>" >&2
  exit 1
fi

START_FILE="$1"
END_FILE="$2"

if [ ! -f "${START_FILE}" ]; then
  echo "Missing start snapshot file: ${START_FILE}" >&2
  exit 1
fi
if [ ! -f "${END_FILE}" ]; then
  echo "Missing end snapshot file: ${END_FILE}" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${START_FILE}"
start_proxy_queue_depth="${proxy_queue_depth}"
start_scrapesite_queue_depth="${scrapesite_queue_depth}"
start_proxy_statistics_count="${proxy_statistics_count}"
start_proxy_statistics_response_rows="${proxy_statistics_response_rows}"
start_proxy_statistics_total_bytes="${proxy_statistics_total_bytes}"
start_database_total_bytes="${database_total_bytes}"
start_timestamp_utc="${timestamp_utc}"

# shellcheck disable=SC1090
source "${END_FILE}"
end_proxy_queue_depth="${proxy_queue_depth}"
end_scrapesite_queue_depth="${scrapesite_queue_depth}"
end_proxy_statistics_count="${proxy_statistics_count}"
end_proxy_statistics_response_rows="${proxy_statistics_response_rows}"
end_proxy_statistics_total_bytes="${proxy_statistics_total_bytes}"
end_database_total_bytes="${database_total_bytes}"
end_timestamp_utc="${timestamp_utc}"
set +a

max_proxy_queue_delta="${PERF_MAX_PROXY_QUEUE_DEPTH_DELTA:-50000}"
max_scrapesite_queue_delta="${PERF_MAX_SCRAPESITE_QUEUE_DEPTH_DELTA:-5000}"
max_proxy_statistics_delta="${PERF_MAX_PROXY_STATISTICS_ROW_DELTA:-25000000}"
max_response_rows_delta="${PERF_MAX_PROXY_STATISTICS_RESPONSE_ROWS_DELTA:-250000}"
max_proxy_statistics_bytes_delta="${PERF_MAX_PROXY_STATISTICS_BYTES_DELTA:-16106127360}"
max_database_bytes_delta="${PERF_MAX_DATABASE_BYTES_DELTA:-21474836480}"

proxy_queue_delta="$((end_proxy_queue_depth - start_proxy_queue_depth))"
scrapesite_queue_delta="$((end_scrapesite_queue_depth - start_scrapesite_queue_depth))"
proxy_statistics_delta="$((end_proxy_statistics_count - start_proxy_statistics_count))"
response_rows_delta="$((end_proxy_statistics_response_rows - start_proxy_statistics_response_rows))"
proxy_statistics_bytes_delta="$((end_proxy_statistics_total_bytes - start_proxy_statistics_total_bytes))"
database_bytes_delta="$((end_database_total_bytes - start_database_total_bytes))"

echo "Snapshot window: ${start_timestamp_utc} -> ${end_timestamp_utc}"
echo "proxy_queue_delta=${proxy_queue_delta}"
echo "scrapesite_queue_delta=${scrapesite_queue_delta}"
echo "proxy_statistics_delta=${proxy_statistics_delta}"
echo "proxy_statistics_response_rows_delta=${response_rows_delta}"
echo "proxy_statistics_bytes_delta=${proxy_statistics_bytes_delta}"
echo "database_bytes_delta=${database_bytes_delta}"

fail=0

if [ "${proxy_queue_delta}" -gt "${max_proxy_queue_delta}" ]; then
  echo "FAIL: proxy queue grew by ${proxy_queue_delta}, budget is ${max_proxy_queue_delta}" >&2
  fail=1
fi

if [ "${scrapesite_queue_delta}" -gt "${max_scrapesite_queue_delta}" ]; then
  echo "FAIL: scrape-site queue grew by ${scrapesite_queue_delta}, budget is ${max_scrapesite_queue_delta}" >&2
  fail=1
fi

if [ "${proxy_statistics_delta}" -gt "${max_proxy_statistics_delta}" ]; then
  echo "FAIL: proxy_statistics rows grew by ${proxy_statistics_delta}, budget is ${max_proxy_statistics_delta}" >&2
  fail=1
fi

if [ "${response_rows_delta}" -gt "${max_response_rows_delta}" ]; then
  echo "FAIL: response_body rows grew by ${response_rows_delta}, budget is ${max_response_rows_delta}" >&2
  fail=1
fi

if [ "${proxy_statistics_bytes_delta}" -gt "${max_proxy_statistics_bytes_delta}" ]; then
  echo "FAIL: proxy_statistics table bytes grew by ${proxy_statistics_bytes_delta}, budget is ${max_proxy_statistics_bytes_delta}" >&2
  fail=1
fi

if [ "${database_bytes_delta}" -gt "${max_database_bytes_delta}" ]; then
  echo "FAIL: database bytes grew by ${database_bytes_delta}, budget is ${max_database_bytes_delta}" >&2
  fail=1
fi

if [ "${fail}" -ne 0 ]; then
  exit 1
fi

echo "PASS: snapshot growth budgets are within configured limits."
