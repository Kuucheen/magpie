export interface UserSettings {
  http_protocol:     boolean
  https_protocol:    boolean
  socks4_protocol:   boolean
  socks5_protocol:   boolean
  timeout:          number
  retries:          number
  UseHttpsForSocks: boolean
  auto_remove_failing_proxies: boolean
  auto_remove_failure_threshold: number

  judges: [{
    url: string
    regex: string
  }]

  scraping_sources: string[]
}
