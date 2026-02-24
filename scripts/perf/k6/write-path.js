import http from "k6/http";
import { check } from "k6";
import { Counter } from "k6/metrics";

import {
  ensureAuthToken,
  envInt,
  generateProxyBatch,
  parseJSON,
  resolveAPIBaseURL,
  authHeaders,
} from "./common.js";

const duration = __ENV.PERF_DURATION || "30m";
const batchSize = envInt("PERF_PROXY_BATCH_SIZE", 250);

export const options = {
  scenarios: {
    write_add_proxies: {
      executor: "constant-arrival-rate",
      exec: "writeAddProxies",
      rate: envInt("PERF_RATE_ADD_PROXIES", 2),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_VUS_ADD_PROXIES", 10),
      maxVUs: envInt("PERF_MAX_VUS_ADD_PROXIES", 80),
    },
  },
  thresholds: {
    "http_req_failed{scenario:write_add_proxies}": ["rate<0.02"],
    "http_req_duration{scenario:write_add_proxies}": ["p(95)<2500", "p(99)<5000"],
  },
};

const submittedProxies = new Counter("perf_submitted_proxies");
const acceptedProxies = new Counter("perf_accepted_proxies");

export function setup() {
  return {
    apiBase: resolveAPIBaseURL(),
    token: ensureAuthToken(),
  };
}

export function writeAddProxies(data) {
  const seed = (__VU * 1_000_000) + (__ITER * batchSize);
  const payload = generateProxyBatch(batchSize, seed);
  const file = http.file(payload, `proxies-${__VU}-${__ITER}.txt`, "text/plain");

  const response = http.post(
    `${data.apiBase}/addProxies`,
    { file },
    { headers: authHeaders(data.token) },
  );

  const body = parseJSON(response);
  const proxyCount = Number.isFinite(body?.proxyCount) ? body.proxyCount : 0;

  submittedProxies.add(batchSize);
  acceptedProxies.add(proxyCount);

  check(response, {
    "write_add_proxies: status 200": (r) => r.status === 200,
    "write_add_proxies: response has proxyCount": () => Number.isFinite(body?.proxyCount),
    "write_add_proxies: response has details": () => typeof body?.details === "object",
  });
}
