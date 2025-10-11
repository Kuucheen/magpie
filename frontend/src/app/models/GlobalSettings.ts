export interface GlobalSettings {
  protocols: {
    http: boolean;
    https: boolean;
    socks4: boolean;
    socks5: boolean;
  };

  checker: {
    dynamic_threads: boolean;
    threads: number;
    retries: number;
    timeout: number;
    checker_timer: {
      days: number;
      hours: number;
      minutes: number;
      seconds: number;
    };
    judges_threads: number;
    judges_timeout: number;
    judges: {
      url: string;
      regex: string;
    }[];
    judge_timer: {
      days: number;
      hours: number;
      minutes: number;
      seconds: number;
    };
    use_https_for_socks: boolean;
    ip_lookup: string;
    standard_header: string[];
    proxy_header: string[];
  };

  scraper: {
    dynamic_threads: boolean;
    threads: number;
    retries: number;
    timeout: number;

    scraper_timer: {
      days: number;
      hours: number;
      minutes: number;
      seconds: number;
    };

    scrape_sites: string[];
  };

  proxy_limits: {
    enabled: boolean;
    max_per_user: number;
    exclude_admins: boolean;
  };

  blacklist_sources: string[];
}
