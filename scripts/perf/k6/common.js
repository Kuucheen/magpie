import http from "k6/http";
import { check, fail } from "k6";

const DEFAULT_PASSWORD = "MagpiePerfPass!123";

export function envInt(name, fallback) {
  const raw = __ENV[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  if (Number.isNaN(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

export function envBool(name, fallback) {
  const raw = (__ENV[name] || "").trim().toLowerCase();
  if (!raw) {
    return fallback;
  }
  if (raw === "1" || raw === "true" || raw === "yes" || raw === "on") {
    return true;
  }
  if (raw === "0" || raw === "false" || raw === "no" || raw === "off") {
    return false;
  }
  return fallback;
}

export function resolveAPIBaseURL() {
  const raw = (__ENV.MAGPIE_BASE_URL || "http://localhost:5656").trim().replace(/\/+$/, "");
  return `${raw}/api`;
}

export function authHeaders(token, extra = {}) {
  return {
    Authorization: `Bearer ${token}`,
    ...extra,
  };
}

export function parseJSON(response) {
  if (!response || !response.body) {
    return null;
  }
  try {
    return JSON.parse(response.body);
  } catch (_) {
    return null;
  }
}

function ensureTokenField(response, payload, contextLabel) {
  const ok = check(response, {
    [`${contextLabel}: status ok`]: (r) => r.status >= 200 && r.status < 300,
    [`${contextLabel}: token returned`]: () => typeof payload?.token === "string" && payload.token.length > 0,
  });
  if (!ok) {
    fail(`${contextLabel} failed: status=${response.status} body=${response.body}`);
  }
  return payload.token;
}

export function ensureAuthToken() {
  if (__ENV.MAGPIE_TOKEN && __ENV.MAGPIE_TOKEN.trim() !== "") {
    return __ENV.MAGPIE_TOKEN.trim();
  }

  const apiBase = resolveAPIBaseURL();
  const password = __ENV.MAGPIE_USER_PASSWORD || DEFAULT_PASSWORD;
  const email = __ENV.MAGPIE_USER_EMAIL || `perf_${Date.now()}@magpie.local`;
  const shouldRegister = envBool("MAGPIE_REGISTER_IF_MISSING", true);

  const headers = { headers: { "Content-Type": "application/json" } };
  const loginBody = JSON.stringify({ email, password });
  const loginResponse = http.post(`${apiBase}/login`, loginBody, headers);

  if (loginResponse.status >= 200 && loginResponse.status < 300) {
    return ensureTokenField(loginResponse, parseJSON(loginResponse), "login");
  }

  if (!shouldRegister) {
    fail(
      `Login failed for ${email} with status ${loginResponse.status}. ` +
        "Set MAGPIE_TOKEN, fix credentials, or enable MAGPIE_REGISTER_IF_MISSING=true.",
    );
  }

  const registerBody = JSON.stringify({ email, password });
  const registerResponse = http.post(`${apiBase}/register`, registerBody, headers);
  if (registerResponse.status >= 200 && registerResponse.status < 300) {
    return ensureTokenField(registerResponse, parseJSON(registerResponse), "register");
  }

  fail(
    `Registration failed for ${email}: status=${registerResponse.status} body=${registerResponse.body}. ` +
      "Set MAGPIE_TOKEN or MAGPIE_USER_EMAIL/MAGPIE_USER_PASSWORD to an existing account.",
  );
}

export function fetchSampleProxyID(apiBase, token) {
  const response = http.get(`${apiBase}/getProxyPage/1?pageSize=50`, {
    headers: authHeaders(token),
  });
  if (response.status !== 200) {
    return null;
  }
  const payload = parseJSON(response);
  if (!payload || !Array.isArray(payload.proxies) || payload.proxies.length === 0) {
    return null;
  }
  const sampleID = payload.proxies[0]?.id;
  if (typeof sampleID !== "number") {
    return null;
  }
  return sampleID;
}

export function graphqlRequest(apiBase, token, query, variables = {}) {
  return http.post(
    `${apiBase}/graphql`,
    JSON.stringify({ query, variables }),
    {
      headers: authHeaders(token, { "Content-Type": "application/json" }),
    },
  );
}

export function generateProxyBatch(size, seed) {
  const lines = [];
  for (let i = 0; i < size; i += 1) {
    const n = seed + i;
    const octet1 = 11 + (n % 210);
    const octet2 = (n >> 8) & 255;
    const octet3 = (n >> 16) & 255;
    const octet4 = (n >> 24) & 255;
    const port = 10000 + (n % 50000);
    lines.push(`${octet1}.${octet2}.${octet3}.${octet4}:${port}`);
  }
  return lines.join("\n");
}
