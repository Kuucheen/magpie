export interface ScrapeSourceDeleteSettings {
  scrapeSources: number[];
  filter: boolean;
  http: boolean;
  https: boolean;
  proxyCountOperator: '<' | '>';
  proxyCount: number;
  aliveCountOperator: '<' | '>';
  aliveCount: number;
  scope: 'all' | 'selected';
}
