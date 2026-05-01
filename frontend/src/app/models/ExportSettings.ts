export interface ExportSettings {
  proxies: number[]
  filter: boolean
  http: boolean
  https: boolean
  socks4: boolean
  socks5: boolean
  minHealthOverall: number
  minHealthHttp: number
  minHealthHttps: number
  minHealthSocks4: number
  minHealthSocks5: number
  maxRetries: number
  maxTimeout: number
  countries: string[]
  types: string[]
  anonymityLevels: string[]
  proxyStatus: 'all' | 'alive' | 'dead'
  reputationLabels: string[]
  outputFormat: string
}
