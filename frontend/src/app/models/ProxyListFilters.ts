export interface ProxyListFilters {
  status?: 'alive' | 'dead';
  protocols?: string[];
  countries?: string[];
  types?: string[];
  anonymityLevels?: string[];
  maxTimeout?: number;
  maxRetries?: number;
  reputationLabels?: string[];
}
