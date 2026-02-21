export type ProxyTableColumnId =
  | 'alive'
  | 'health_overall'
  | 'health_http'
  | 'health_https'
  | 'health_socks4'
  | 'health_socks5'
  | 'ip'
  | 'ip_port'
  | 'port'
  | 'response_time'
  | 'estimated_type'
  | 'country'
  | 'reputation'
  | 'latest_check'
  | 'actions';

export interface ProxyTableColumnDefinition {
  id: ProxyTableColumnId;
  label: string;
  sortField?: string;
  tooltip?: string;
  example?: string;
  skeletonWidth?: string;
}

export const PROXY_TABLE_COLUMN_DEFINITIONS: readonly ProxyTableColumnDefinition[] = [
  {
    id: 'alive',
    label: 'Status',
    sortField: 'alive',
    example: 'alive',
    skeletonWidth: '0.9rem',
  },
  {
    id: 'health_overall',
    label: 'Overall Health',
    sortField: 'health_overall',
    tooltip: 'Health ratio across all checks',
    example: '82%',
    skeletonWidth: '3.5rem',
  },
  {
    id: 'health_http',
    label: 'HTTP Health',
    sortField: 'health_http',
    tooltip: 'Health ratio for HTTP checks',
    example: '79%',
    skeletonWidth: '3.5rem',
  },
  {
    id: 'health_https',
    label: 'HTTPS Health',
    sortField: 'health_https',
    tooltip: 'Health ratio for HTTPS checks',
    example: '85%',
    skeletonWidth: '3.75rem',
  },
  {
    id: 'health_socks4',
    label: 'SOCKS4 Health',
    sortField: 'health_socks4',
    tooltip: 'Health ratio for SOCKS4 checks',
    example: '68%',
    skeletonWidth: '4rem',
  },
  {
    id: 'health_socks5',
    label: 'SOCKS5 Health',
    sortField: 'health_socks5',
    tooltip: 'Health ratio for SOCKS5 checks',
    example: '71%',
    skeletonWidth: '4rem',
  },
  {
    id: 'ip',
    label: 'IP Address',
    sortField: 'ip',
    example: '127.0.0.1',
    skeletonWidth: '7rem',
  },
  {
    id: 'ip_port',
    label: 'IP:Port',
    sortField: 'ip_port',
    example: '127.0.0.1:8080',
    skeletonWidth: '10rem',
  },
  {
    id: 'port',
    label: 'Port',
    sortField: 'port',
    example: '8080',
    skeletonWidth: '3rem',
  },
  {
    id: 'response_time',
    label: 'Time',
    sortField: 'response_time',
    tooltip: 'Response Time',
    example: '120 ms',
    skeletonWidth: '4rem',
  },
  {
    id: 'estimated_type',
    label: 'Type',
    sortField: 'estimated_type',
    tooltip: 'Estimated Type',
    example: 'HTTP',
    skeletonWidth: '5rem',
  },
  {
    id: 'country',
    label: 'Country',
    sortField: 'country',
    example: 'US',
    skeletonWidth: '6rem',
  },
  {
    id: 'reputation',
    label: 'Reputation',
    sortField: 'reputation',
    example: 'Good (82)',
    skeletonWidth: '3.5rem',
  },
  {
    id: 'latest_check',
    label: 'Last Check',
    sortField: 'latest_check',
    example: '2026-02-20 10:30',
    skeletonWidth: '6rem',
  },
  {
    id: 'actions',
    label: 'Actions',
    example: 'Details',
    skeletonWidth: '4.5rem',
  },
];

export const DEFAULT_PROXY_TABLE_COLUMNS: readonly ProxyTableColumnId[] = [
  'alive',
  'health_overall',
  'health_http',
  'health_https',
  'health_socks4',
  'health_socks5',
  'ip_port',
  'response_time',
  'estimated_type',
  'country',
  'reputation',
  'latest_check',
  'actions',
];

const proxyColumnById = new Map<ProxyTableColumnId, ProxyTableColumnDefinition>(
  PROXY_TABLE_COLUMN_DEFINITIONS.map(column => [column.id, column] as const)
);

const LEGACY_PROXY_COLUMN_ID_MAP: Readonly<Record<string, ProxyTableColumnId>> = {
  alive_ratio_overall: 'health_overall',
  alive_ratio_http: 'health_http',
  alive_ratio_https: 'health_https',
  alive_ratio_socks4: 'health_socks4',
  alive_ratio_socks5: 'health_socks5',
};

export function getProxyTableColumnDefinition(id: ProxyTableColumnId): ProxyTableColumnDefinition {
  return proxyColumnById.get(id) ?? proxyColumnById.get('ip')!;
}

export function normalizeProxyTableColumns(value: unknown): ProxyTableColumnId[] {
  if (!Array.isArray(value)) {
    return [...DEFAULT_PROXY_TABLE_COLUMNS];
  }

  const seen = new Set<ProxyTableColumnId>();
  const normalized: ProxyTableColumnId[] = [];

  for (const item of value) {
    if (typeof item !== 'string') {
      continue;
    }
    const candidate = (LEGACY_PROXY_COLUMN_ID_MAP[item] ?? item) as ProxyTableColumnId;
    if (!proxyColumnById.has(candidate) || seen.has(candidate)) {
      continue;
    }
    seen.add(candidate);
    normalized.push(candidate);
  }

  if (normalized.length === 0) {
    return [...DEFAULT_PROXY_TABLE_COLUMNS];
  }

  return normalized;
}
