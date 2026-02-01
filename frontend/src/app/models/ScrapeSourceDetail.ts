export interface ScrapeSourceReputationBreakdown {
  good: number;
  neutral: number;
  poor: number;
  unknown: number;
}

export interface ScrapeSourceDetail {
  id: number;
  url: string;
  added_at: string;
  proxy_count: number;
  alive_count: number;
  dead_count: number;
  unknown_count: number;
  avg_reputation?: number | null;
  last_proxy_added_at?: string | null;
  last_checked_at?: string | null;
  reputation_breakdown: ScrapeSourceReputationBreakdown;
}
