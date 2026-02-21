import {ProxyReputationSummary} from './ProxyReputation';

export interface ProxyHealthSummary {
  "overall"?: number;
  "http"?: number;
  "https"?: number;
  "socks4"?: number;
  "socks5"?: number;
}

export interface ProxyInfo {
  "id": number;
  "ip": string;
  "port": number;
  "estimated_type": string;
  "response_time": number;
  "country": string;
  "anonymity_level": string;
  "alive": boolean;
  "health"?: ProxyHealthSummary | null;
  "latest_check": Date;
  "reputation"?: ProxyReputationSummary | null;
}

export interface ProxyPage {
  "proxies": ProxyInfo[];
  "total": number;
}
