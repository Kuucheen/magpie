import http from "k6/http";
import { check } from "k6";
import { Counter, Rate } from "k6/metrics";

import {
  ensureAuthToken,
  envInt,
  fetchSampleProxyID,
  generateProxyBatch,
  graphqlRequest,
  parseJSON,
  resolveAPIBaseURL,
  authHeaders,
} from "./common.js";

const duration = __ENV.PERF_SOAK_DURATION || "2h";
const writeBatchSize = envInt("PERF_SOAK_PROXY_BATCH_SIZE", 100);

export const options = {
  scenarios: {
    soak_read_proxy_page: {
      executor: "constant-arrival-rate",
      exec: "soakReadProxyPage",
      rate: envInt("PERF_SOAK_RATE_PROXY_PAGE", 20),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_SOAK_VUS_PROXY_PAGE", 30),
      maxVUs: envInt("PERF_SOAK_MAX_VUS_PROXY_PAGE", 300),
    },
    soak_graphql_viewer: {
      executor: "constant-arrival-rate",
      exec: "soakGraphqlViewer",
      rate: envInt("PERF_SOAK_RATE_GRAPHQL", 8),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_SOAK_VUS_GRAPHQL", 20),
      maxVUs: envInt("PERF_SOAK_MAX_VUS_GRAPHQL", 200),
    },
    soak_proxy_stats: {
      executor: "constant-arrival-rate",
      exec: "soakProxyStatistics",
      rate: envInt("PERF_SOAK_RATE_PROXY_STATS", 4),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_SOAK_VUS_PROXY_STATS", 10),
      maxVUs: envInt("PERF_SOAK_MAX_VUS_PROXY_STATS", 100),
    },
    soak_write_add_proxies: {
      executor: "constant-arrival-rate",
      exec: "soakWriteAddProxies",
      rate: envInt("PERF_SOAK_RATE_ADD_PROXIES", 1),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_SOAK_VUS_ADD_PROXIES", 8),
      maxVUs: envInt("PERF_SOAK_MAX_VUS_ADD_PROXIES", 80),
    },
  },
  thresholds: {
    "http_req_failed{scenario:soak_read_proxy_page}": ["rate<0.01"],
    "http_req_duration{scenario:soak_read_proxy_page}": ["p(95)<1000", "p(99)<2200"],

    "http_req_failed{scenario:soak_graphql_viewer}": ["rate<0.01"],
    "http_req_duration{scenario:soak_graphql_viewer}": ["p(95)<1200", "p(99)<2400"],

    "http_req_failed{scenario:soak_proxy_stats}": ["rate<0.01"],
    "http_req_duration{scenario:soak_proxy_stats}": ["p(95)<1000", "p(99)<2200"],

    "http_req_failed{scenario:soak_write_add_proxies}": ["rate<0.02"],
    "http_req_duration{scenario:soak_write_add_proxies}": ["p(95)<3000", "p(99)<6000"],
  },
};

const submittedProxies = new Counter("perf_soak_submitted_proxies");
const acceptedProxies = new Counter("perf_soak_accepted_proxies");
const missingSampleProxyRate = new Rate("perf_soak_missing_sample_proxy");

export function setup() {
  const apiBase = resolveAPIBaseURL();
  const token = ensureAuthToken();
  const sampleProxyID = fetchSampleProxyID(apiBase, token);
  return {
    apiBase,
    token,
    sampleProxyID,
  };
}

export function soakReadProxyPage(data) {
  const response = http.get(`${data.apiBase}/getProxyPage/1?pageSize=100`, {
    headers: authHeaders(data.token),
  });
  const body = parseJSON(response);
  check(response, {
    "soak_read_proxy_page: status 200": (r) => r.status === 200,
    "soak_read_proxy_page: proxies array": () => Array.isArray(body?.proxies),
  });
}

const viewerQuery = `query SoakViewerQuery { viewer { id role proxyCount } }`;

export function soakGraphqlViewer(data) {
  const response = graphqlRequest(data.apiBase, data.token, viewerQuery);
  const body = parseJSON(response);
  check(response, {
    "soak_graphql_viewer: status 200": (r) => r.status === 200,
    "soak_graphql_viewer: has viewer": () => body?.data?.viewer !== undefined,
    "soak_graphql_viewer: no errors": () => Array.isArray(body?.errors) === false,
  });
}

export function soakProxyStatistics(data) {
  if (!data.sampleProxyID) {
    missingSampleProxyRate.add(1);
    const fallback = http.get(`${data.apiBase}/getProxyPage/1?pageSize=1`, {
      headers: authHeaders(data.token),
    });
    check(fallback, {
      "soak_proxy_stats: fallback status 200": (r) => r.status === 200,
    });
    return;
  }

  missingSampleProxyRate.add(0);
  const response = http.get(`${data.apiBase}/proxies/${data.sampleProxyID}/statistics?limit=100`, {
    headers: authHeaders(data.token),
  });
  const body = parseJSON(response);
  check(response, {
    "soak_proxy_stats: status 200": (r) => r.status === 200,
    "soak_proxy_stats: statistics array": () => Array.isArray(body?.statistics),
  });
}

export function soakWriteAddProxies(data) {
  const seed = (__VU * 1_000_000) + (__ITER * writeBatchSize);
  const proxyBatch = generateProxyBatch(writeBatchSize, seed);
  const file = http.file(proxyBatch, `soak-${__VU}-${__ITER}.txt`, "text/plain");

  const response = http.post(
    `${data.apiBase}/addProxies`,
    { file },
    { headers: authHeaders(data.token) },
  );

  const body = parseJSON(response);
  const proxyCount = Number.isFinite(body?.proxyCount) ? body.proxyCount : 0;
  submittedProxies.add(writeBatchSize);
  acceptedProxies.add(proxyCount);

  check(response, {
    "soak_write_add_proxies: status 200": (r) => r.status === 200,
    "soak_write_add_proxies: response has proxyCount": () => Number.isFinite(body?.proxyCount),
  });
}
