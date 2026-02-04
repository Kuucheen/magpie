import { FormGroup } from '@angular/forms';
import { ProxyFilterOptions } from '../models/ProxyFilterOptions';
import { ProxyListFilters } from '../models/ProxyListFilters';

export type ProxyFilterOption = {
  label: string;
  value: string;
};

export type ProxyListFilterFormValues = {
  proxyStatus: 'all' | 'alive' | 'dead';
  http: boolean;
  https: boolean;
  socks4: boolean;
  socks5: boolean;
  maxTimeout: number;
  maxRetries: number;
  countries: string[];
  types: string[];
  anonymityLevels: string[];
  reputationLabels: string[];
};

export type ProxyListAppliedFilters = {
  status: 'all' | 'alive' | 'dead';
  protocols: string[];
  maxTimeout: number;
  maxRetries: number;
  countries: string[];
  types: string[];
  anonymityLevels: string[];
  reputationLabels: string[];
};

export const PROXY_STATUS_OPTIONS: ProxyFilterOption[] = [
  { label: 'All Proxies', value: 'all' },
  { label: 'Only Alive Proxies', value: 'alive' },
  { label: 'Only Dead Proxies', value: 'dead' },
];

export const PROXY_REPUTATION_OPTIONS: ProxyFilterOption[] = [
  { label: 'Good', value: 'good' },
  { label: 'Neutral', value: 'neutral' },
  { label: 'Poor', value: 'poor' },
  { label: 'Unknown', value: 'unknown' },
];

export function createDefaultProxyFilterValues(): ProxyListFilterFormValues {
  return {
    proxyStatus: 'all',
    http: false,
    https: false,
    socks4: false,
    socks5: false,
    maxTimeout: 0,
    maxRetries: 0,
    countries: [],
    types: [],
    anonymityLevels: [],
    reputationLabels: [],
  };
}

export function createDefaultProxyListAppliedFilters(): ProxyListAppliedFilters {
  return {
    status: 'all',
    protocols: [],
    maxTimeout: 0,
    maxRetries: 0,
    countries: [],
    types: [],
    anonymityLevels: [],
    reputationLabels: [],
  };
}

export function activeProxyFilterCount(filters: ProxyListAppliedFilters): number {
  let count = 0;
  if (filters.status !== 'all') {
    count += 1;
  }
  if (filters.protocols.length > 0) {
    count += 1;
  }
  if (filters.countries.length > 0) {
    count += 1;
  }
  if (filters.types.length > 0) {
    count += 1;
  }
  if (filters.anonymityLevels.length > 0) {
    count += 1;
  }
  if (filters.maxTimeout > 0) {
    count += 1;
  }
  if (filters.maxRetries > 0) {
    count += 1;
  }
  if (filters.reputationLabels.length > 0) {
    count += 1;
  }
  return count;
}

export function normalizeFilterOptions(options: ProxyFilterOptions): ProxyFilterOptions {
  return {
    countries: sortFilterOptions(options.countries),
    types: sortFilterOptions(options.types),
    anonymityLevels: sortFilterOptions(options.anonymityLevels),
  };
}

export function sortFilterOptions(values: string[]): string[] {
  const cleaned = (values ?? []).filter(value => value && value.trim().length > 0);
  cleaned.sort((a, b) => {
    if (a === 'N/A') {
      return 1;
    }
    if (b === 'N/A') {
      return -1;
    }
    return a.localeCompare(b);
  });
  return cleaned;
}

export function buildFilterOptionList(values: string[]): ProxyFilterOption[] {
  return (values ?? []).map(value => ({ label: value, value }));
}

export function syncFilterFormWithApplied(form: FormGroup, filters: ProxyListAppliedFilters): void {
  form.patchValue({
    proxyStatus: filters.status,
    http: filters.protocols.includes('http'),
    https: filters.protocols.includes('https'),
    socks4: filters.protocols.includes('socks4'),
    socks5: filters.protocols.includes('socks5'),
    maxTimeout: filters.maxTimeout ?? 0,
    maxRetries: filters.maxRetries ?? 0,
    countries: [...filters.countries],
    types: [...filters.types],
    anonymityLevels: [...filters.anonymityLevels],
    reputationLabels: [...filters.reputationLabels],
  }, { emitEvent: false });
}

export function buildFiltersFromFormValue(formValue: ProxyListFilterFormValues): ProxyListAppliedFilters {
  const protocols: string[] = [];
  if (formValue.http) {
    protocols.push('http');
  }
  if (formValue.https) {
    protocols.push('https');
  }
  if (formValue.socks4) {
    protocols.push('socks4');
  }
  if (formValue.socks5) {
    protocols.push('socks5');
  }

  return {
    status: formValue.proxyStatus ?? 'all',
    protocols,
    maxTimeout: normalizeNumber(formValue.maxTimeout),
    maxRetries: normalizeNumber(formValue.maxRetries),
    countries: normalizeSelection(formValue.countries),
    types: normalizeSelection(formValue.types),
    anonymityLevels: normalizeSelection(formValue.anonymityLevels),
    reputationLabels: normalizeSelection(formValue.reputationLabels),
  };
}

export function buildProxyListFilterPayload(filters: ProxyListAppliedFilters): ProxyListFilters | undefined {
  const payload: ProxyListFilters = {};

  if (filters.status !== 'all') {
    payload.status = filters.status;
  }
  if (filters.protocols.length > 0) {
    payload.protocols = filters.protocols;
  }
  if (filters.countries.length > 0) {
    payload.countries = filters.countries;
  }
  if (filters.types.length > 0) {
    payload.types = filters.types;
  }
  if (filters.anonymityLevels.length > 0) {
    payload.anonymityLevels = filters.anonymityLevels;
  }
  if (filters.maxTimeout > 0) {
    payload.maxTimeout = filters.maxTimeout;
  }
  if (filters.maxRetries > 0) {
    payload.maxRetries = filters.maxRetries;
  }
  if (filters.reputationLabels.length > 0) {
    payload.reputationLabels = filters.reputationLabels;
  }

  return Object.keys(payload).length > 0 ? payload : undefined;
}

export function normalizeSelection(values: string[] | null | undefined): string[] {
  if (!values || values.length === 0) {
    return [];
  }
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const value of values) {
    const trimmed = `${value}`.trim();
    if (!trimmed || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    normalized.push(trimmed);
  }
  return normalized;
}

export function normalizeNumber(value: number | string | null | undefined): number {
  if (value === null || value === undefined) {
    return 0;
  }
  const parsed = typeof value === 'string' ? Number(value) : value;
  if (!Number.isFinite(parsed)) {
    return 0;
  }
  return Math.max(0, Math.floor(parsed));
}
