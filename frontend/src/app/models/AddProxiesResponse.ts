export interface AddProxiesDetails {
  submittedCount: number;
  parsedCount: number;
  invalidFormatCount: number;
  invalidIpCount: number;
  invalidIpv4Count: number;
  invalidPortCount: number;
  blacklistedCount: number;
  processingMs: number;
}

export interface AddProxiesResponse {
  proxyCount: number;
  details: AddProxiesDetails;
}
