export interface ScrapeSourceListFilters {
  protocols?: string[];
  proxyCountOperator?: '<' | '>';
  proxyCount?: number;
  aliveCountOperator?: '<' | '>';
  aliveCount?: number;
}
