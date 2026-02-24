import http from "k6/http";
import { check } from "k6";
import { Rate } from "k6/metrics";

import {
  ensureAuthToken,
  envInt,
  fetchSampleProxyID,
  graphqlRequest,
  parseJSON,
  resolveAPIBaseURL,
  authHeaders,
} from "./common.js";

const duration = __ENV.PERF_DURATION || "30m";

export const options = {
  scenarios: {
    rest_proxy_page: {
      executor: "constant-arrival-rate",
      exec: "restProxyPage",
      rate: envInt("PERF_RATE_PROXY_PAGE", 30),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_VUS_PROXY_PAGE", 40),
      maxVUs: envInt("PERF_MAX_VUS_PROXY_PAGE", 400),
    },
    rest_proxy_filters: {
      executor: "constant-arrival-rate",
      exec: "restProxyFilters",
      rate: envInt("PERF_RATE_PROXY_FILTERS", 10),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_VUS_PROXY_FILTERS", 20),
      maxVUs: envInt("PERF_MAX_VUS_PROXY_FILTERS", 200),
    },
    rest_dashboard: {
      executor: "constant-arrival-rate",
      exec: "restDashboard",
      rate: envInt("PERF_RATE_DASHBOARD", 10),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_VUS_DASHBOARD", 20),
      maxVUs: envInt("PERF_MAX_VUS_DASHBOARD", 200),
    },
    graphql_viewer: {
      executor: "constant-arrival-rate",
      exec: "graphqlViewer",
      rate: envInt("PERF_RATE_GRAPHQL_VIEWER", 10),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_VUS_GRAPHQL_VIEWER", 20),
      maxVUs: envInt("PERF_MAX_VUS_GRAPHQL_VIEWER", 200),
    },
    rest_proxy_statistics: {
      executor: "constant-arrival-rate",
      exec: "restProxyStatistics",
      rate: envInt("PERF_RATE_PROXY_STATS", 5),
      timeUnit: "1s",
      duration,
      preAllocatedVUs: envInt("PERF_VUS_PROXY_STATS", 10),
      maxVUs: envInt("PERF_MAX_VUS_PROXY_STATS", 100),
    },
  },
  thresholds: {
    "http_req_failed{scenario:rest_proxy_page}": ["rate<0.01"],
    "http_req_duration{scenario:rest_proxy_page}": ["p(95)<900", "p(99)<1800"],

    "http_req_failed{scenario:rest_proxy_filters}": ["rate<0.01"],
    "http_req_duration{scenario:rest_proxy_filters}": ["p(95)<700", "p(99)<1500"],

    "http_req_failed{scenario:rest_dashboard}": ["rate<0.01"],
    "http_req_duration{scenario:rest_dashboard}": ["p(95)<900", "p(99)<1800"],

    "http_req_failed{scenario:graphql_viewer}": ["rate<0.01"],
    "http_req_duration{scenario:graphql_viewer}": ["p(95)<1000", "p(99)<2000"],

    "http_req_failed{scenario:rest_proxy_statistics}": ["rate<0.01"],
    "http_req_duration{scenario:rest_proxy_statistics}": ["p(95)<900", "p(99)<1800"],
  },
};

const missingSampleProxyRate = new Rate("perf_missing_sample_proxy");

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

export function restProxyPage(data) {
  const response = http.get(`${data.apiBase}/getProxyPage/1?pageSize=100`, {
    headers: authHeaders(data.token),
  });
  const payload = parseJSON(response);
  check(response, {
    "rest_proxy_page: status 200": (r) => r.status === 200,
    "rest_proxy_page: proxies array": () => Array.isArray(payload?.proxies),
    "rest_proxy_page: total number": () => typeof payload?.total === "number",
  });
}

export function restProxyFilters(data) {
  const response = http.get(`${data.apiBase}/proxyFilters`, {
    headers: authHeaders(data.token),
  });
  check(response, {
    "rest_proxy_filters: status 200": (r) => r.status === 200,
    "rest_proxy_filters: body json": (r) => parseJSON(r) !== null,
  });
}

export function restDashboard(data) {
  const response = http.get(`${data.apiBase}/getDashboardInfo`, {
    headers: authHeaders(data.token),
  });
  check(response, {
    "rest_dashboard: status 200": (r) => r.status === 200,
    "rest_dashboard: body json": (r) => parseJSON(r) !== null,
  });
}

const viewerQuery = `query ViewerPerf { viewer { id role proxyCount } }`;

export function graphqlViewer(data) {
  const response = graphqlRequest(data.apiBase, data.token, viewerQuery);
  const payload = parseJSON(response);
  check(response, {
    "graphql_viewer: status 200": (r) => r.status === 200,
    "graphql_viewer: no errors": () => Array.isArray(payload?.errors) === false,
    "graphql_viewer: has viewer": () => payload?.data?.viewer !== undefined,
  });
}

export function restProxyStatistics(data) {
  if (!data.sampleProxyID) {
    missingSampleProxyRate.add(1);
    const fallback = http.get(`${data.apiBase}/getProxyPage/1?pageSize=1`, {
      headers: authHeaders(data.token),
    });
    check(fallback, {
      "rest_proxy_statistics: fallback status 200": (r) => r.status === 200,
    });
    return;
  }

  missingSampleProxyRate.add(0);
  const response = http.get(`${data.apiBase}/proxies/${data.sampleProxyID}/statistics?limit=100`, {
    headers: authHeaders(data.token),
  });
  const payload = parseJSON(response);
  check(response, {
    "rest_proxy_statistics: status 200": (r) => r.status === 200,
    "rest_proxy_statistics: statistics array": () => Array.isArray(payload?.statistics),
  });
}
