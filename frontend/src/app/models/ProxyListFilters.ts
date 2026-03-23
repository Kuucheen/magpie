export interface ProxyListFilters {
  status?: 'alive' | 'dead';
  protocols?: string[];
  minHealthOverall?: number;
  minHealthHttp?: number;
  minHealthHttps?: number;
  minHealthSocks4?: number;
  minHealthSocks5?: number;
  countries?: string[];
  types?: string[];
  anonymityLevels?: string[];
  maxTimeout?: number;
  maxRetries?: number;
  reputationLabels?: string[];
}
