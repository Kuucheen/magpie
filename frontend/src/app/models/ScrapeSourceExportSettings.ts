export interface ScrapeSourceExportSettings {
  scrapeSources: number[];
  filter: boolean;
  http: boolean;
  https: boolean;
  proxyCountOperator: '<' | '>';
  proxyCount: number;
  aliveCountOperator: '<' | '>';
  aliveCount: number;
  outputFormat: string;
}
