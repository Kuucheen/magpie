import {Component, OnDestroy, OnInit, signal} from '@angular/core';
import {CommonModule, DatePipe, NgClass} from '@angular/common';
import {FormBuilder, FormGroup} from '@angular/forms';
import {ActivatedRoute, Router, RouterLink} from '@angular/router';
import {Subscription} from 'rxjs';
import {ScrapeSourceDetail} from '../../models/ScrapeSourceDetail';
import {HttpService} from '../../services/http.service';
import {NotificationService} from '../../services/notification-service.service';
import {LoadingComponent} from '../../ui-elements/loading/loading.component';
import {ProxyInfo} from '../../models/ProxyInfo';
import {TableLazyLoadEvent} from 'primeng/table';
import {ButtonModule} from 'primeng/button';
import {ProxyListFilters} from '../../models/ProxyListFilters';
import {ProxyFilterOptions} from '../../models/ProxyFilterOptions';
import {ProxyFilterPanelComponent} from '../../shared/proxy-filter-panel/proxy-filter-panel.component';
import {ProxyTableComponent} from '../../shared/proxy-table/proxy-table.component';

type ProxyListFilterFormValues = {
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

type ProxyListAppliedFilters = {
  status: 'all' | 'alive' | 'dead';
  protocols: string[];
  maxTimeout: number;
  maxRetries: number;
  countries: string[];
  types: string[];
  anonymityLevels: string[];
  reputationLabels: string[];
};

type FilterOption = {
  label: string;
  value: string;
};

type HealthTone = 'healthy' | 'mixed' | 'unhealthy' | 'empty';
type ReputationLabel = 'good' | 'neutral' | 'poor' | 'unknown';

@Component({
  selector: 'app-scrape-source-detail',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    DatePipe,
    NgClass,
    LoadingComponent,
    ButtonModule,
    ProxyFilterPanelComponent,
    ProxyTableComponent,
  ],
  templateUrl: './scrape-source-detail.component.html',
  styleUrl: './scrape-source-detail.component.scss'
})
export class ScrapeSourceDetailComponent implements OnInit, OnDestroy {
  sourceId = signal<number | undefined>(undefined);
  detail = signal<ScrapeSourceDetail | null>(null);
  isLoading = signal(true);
  proxies = signal<ProxyInfo[]>([]);
  proxyPage = signal(1);
  proxyPageSize = signal(20);
  proxyTotal = signal(0);
  proxyLoading = signal(false);
  proxyHasLoaded = signal(false);
  proxySearchTerm = signal('');
  filterPanelOpen = signal(false);
  filterOptionsLoaded = signal(false);
  filterOptions = signal<ProxyFilterOptions>({countries: [], types: [], anonymityLevels: []});
  countryOptions = signal<FilterOption[]>([]);
  typeOptions = signal<FilterOption[]>([]);
  anonymityOptions = signal<FilterOption[]>([]);
  appliedFilters = signal<ProxyListAppliedFilters>({
    status: 'all',
    protocols: [],
    maxTimeout: 0,
    maxRetries: 0,
    countries: [],
    types: [],
    anonymityLevels: [],
    reputationLabels: [],
  });
  filterForm: FormGroup;
  readonly proxyStatusOptions = [
    {label: 'All Proxies', value: 'all'},
    {label: 'Only Alive Proxies', value: 'alive'},
    {label: 'Only Dead Proxies', value: 'dead'},
  ];
  readonly proxyReputationOptions = [
    {label: 'Good', value: 'good'},
    {label: 'Neutral', value: 'neutral'},
    {label: 'Poor', value: 'poor'},
    {label: 'Unknown', value: 'unknown'},
  ];
  private readonly defaultFilterValues: ProxyListFilterFormValues = {
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
  readonly proxySkeletonRows = Array.from({ length: 6 });

  private proxySearchDebounce?: ReturnType<typeof setTimeout>;

  private subscriptions = new Subscription();

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private http: HttpService,
    private fb: FormBuilder,
  ) {
    this.filterForm = this.fb.group({
      proxyStatus: [this.defaultFilterValues.proxyStatus],
      http: [this.defaultFilterValues.http],
      https: [this.defaultFilterValues.https],
      socks4: [this.defaultFilterValues.socks4],
      socks5: [this.defaultFilterValues.socks5],
      maxTimeout: [this.defaultFilterValues.maxTimeout],
      maxRetries: [this.defaultFilterValues.maxRetries],
      countries: [this.defaultFilterValues.countries],
      types: [this.defaultFilterValues.types],
      anonymityLevels: [this.defaultFilterValues.anonymityLevels],
      reputationLabels: [this.defaultFilterValues.reputationLabels],
    });
  }

  ngOnInit(): void {
    const sub = this.route.paramMap.subscribe(params => {
      const rawId = params.get('id');
      const id = rawId ? Number(rawId) : NaN;
      if (!Number.isFinite(id) || id <= 0) {
        NotificationService.showError('Invalid scrape source identifier');
        this.router.navigate(['/scraper']).catch(() => {});
        return;
      }

      this.sourceId.set(id);
      this.loadScrapeSourceDetail(id);
      this.proxyPage.set(1);
      this.proxySearchTerm.set('');
      this.proxyHasLoaded.set(false);
      this.proxies.set([]);
      this.proxyTotal.set(0);
      this.filterPanelOpen.set(false);
      this.loadProxyList(id);
    });

    this.subscriptions.add(sub);
  }

  ngOnDestroy(): void {
    if (this.proxySearchDebounce) {
      clearTimeout(this.proxySearchDebounce);
      this.proxySearchDebounce = undefined;
    }
    this.subscriptions.unsubscribe();
  }

  get totalProxies(): number {
    return this.detail()?.proxy_count ?? 0;
  }

  get aliveCount(): number {
    return this.detail()?.alive_count ?? 0;
  }

  get deadCount(): number {
    return this.detail()?.dead_count ?? 0;
  }

  get unknownCount(): number {
    return this.detail()?.unknown_count ?? 0;
  }

  get healthTone(): HealthTone {
    const total = this.totalProxies;
    if (total === 0) {
      return 'empty';
    }
    const ratio = this.aliveCount / total;
    if (ratio >= 0.7) {
      return 'healthy';
    }
    if (ratio >= 0.4) {
      return 'mixed';
    }
    return 'unhealthy';
  }

  get healthLabel(): string {
    switch (this.healthTone) {
      case 'healthy':
        return 'Healthy';
      case 'mixed':
        return 'Mixed';
      case 'unhealthy':
        return 'Unhealthy';
      default:
        return 'No data';
    }
  }

  get healthPillClass(): string {
    switch (this.healthTone) {
      case 'mixed':
        return 'status-pill--mixed';
      case 'unhealthy':
        return 'status-pill--dead';
      case 'empty':
        return 'status-pill--unknown';
      default:
        return '';
    }
  }

  get healthDotClass(): Record<string, boolean> {
    return {
      alive: this.healthTone === 'healthy',
      mixed: this.healthTone === 'mixed',
      dead: this.healthTone === 'unhealthy',
      unknown: this.healthTone === 'empty',
    };
  }

  get aliveRatioDisplay(): string {
    const total = this.totalProxies;
    if (total === 0) {
      return 'No data';
    }
    const ratio = Math.round((this.aliveCount / total) * 100);
    return `${ratio}% alive`;
  }

  get averageReputationScore(): number | null {
    const value = this.detail()?.avg_reputation;
    if (value === null || value === undefined) {
      return null;
    }
    return value;
  }

  get averageReputationDisplay(): string {
    const value = this.averageReputationScore;
    if (value === null) {
      return 'N/A';
    }
    return Math.round(value).toString();
  }

  get averageReputationLabel(): ReputationLabel {
    const value = this.averageReputationScore;
    if (value === null) {
      return 'unknown';
    }
    if (value >= 75) {
      return 'good';
    }
    if (value >= 55) {
      return 'neutral';
    }
    return 'poor';
  }

  get reputationBreakdown() {
    return this.detail()?.reputation_breakdown ?? {
      good: 0,
      neutral: 0,
      poor: 0,
      unknown: 0,
    };
  }

  reputationPercent(label: ReputationLabel): number {
    const breakdown = this.reputationBreakdown;
    const total = breakdown.good + breakdown.neutral + breakdown.poor + breakdown.unknown;
    if (total === 0) {
      return 0;
    }
    return (breakdown[label] / total) * 100;
  }

  healthPercent(count: number): number {
    const total = this.totalProxies;
    if (total === 0) {
      return 0;
    }
    return (count / total) * 100;
  }

  reputationBadgeClass(label: ReputationLabel): string {
    if (label === 'good') {
      return 'reputation-badge reputation-badge--good';
    }
    if (label === 'neutral') {
      return 'reputation-badge reputation-badge--neutral';
    }
    if (label === 'poor') {
      return 'reputation-badge reputation-badge--poor';
    }
    return 'reputation-badge reputation-badge--unknown';
  }

  onProxyLazyLoad(event: TableLazyLoadEvent): void {
    const newPage = Math.floor((event.first ?? 0) / (event.rows ?? this.proxyPageSize())) + 1;
    const newPageSize = event.rows ?? this.proxyPageSize();

    const shouldFetch = newPage !== this.proxyPage() || newPageSize !== this.proxyPageSize();

    this.proxyPage.set(newPage);
    this.proxyPageSize.set(newPageSize);

    if (shouldFetch) {
      this.loadProxyList();
    }
  }

  onProxySearchTermChange(value: string): void {
    this.proxySearchTerm.set(value);
    if (this.proxySearchDebounce) {
      clearTimeout(this.proxySearchDebounce);
    }
    this.proxySearchDebounce = setTimeout(() => {
      this.proxyPage.set(1);
      this.loadProxyList();
    }, 300);
  }

  refreshProxyList(): void {
    if (this.proxySearchDebounce) {
      clearTimeout(this.proxySearchDebounce);
      this.proxySearchDebounce = undefined;
    }
    this.loadProxyList();
  }

  toggleFilterPanel(): void {
    const nextState = !this.filterPanelOpen();
    if (nextState) {
      this.syncFilterFormWithApplied();
      this.ensureFilterOptionsLoaded();
    }
    this.filterPanelOpen.set(nextState);
  }

  applyFilters(): void {
    const nextFilters = this.buildFiltersFromForm();
    this.appliedFilters.set(nextFilters);
    this.proxyPage.set(1);
    this.loadProxyList();
    this.filterPanelOpen.set(false);
  }

  clearFilters(): void {
    this.filterForm.reset(this.defaultFilterValues);
    this.appliedFilters.set({
      status: 'all',
      protocols: [],
      maxTimeout: 0,
      maxRetries: 0,
      countries: [],
      types: [],
      anonymityLevels: [],
      reputationLabels: [],
    });
    this.proxyPage.set(1);
    this.loadProxyList();
  }

  filterButtonLabel(): string {
    const count = this.activeFilterCount();
    if (count === 0) {
      return 'Filters';
    }
    return `Filters (${count})`;
  }

  filterToggleClass(): string {
    if (this.activeFilterCount() > 0) {
      return 'p-button-outlined filter-toggle filter-toggle--active';
    }
    return 'p-button-outlined filter-toggle';
  }

  onViewProxy(proxy: ProxyInfo): void {
    const sourceId = this.sourceId();
    const queryParams = sourceId ? { sourceId } : undefined;
    this.router.navigate(['/proxies', proxy.id], { queryParams }).catch(() => {});
  }

  private loadScrapeSourceDetail(id: number): void {
    this.isLoading.set(true);
    const sub = this.http.getScrapeSourceDetail(id).subscribe({
      next: detail => {
        this.detail.set(detail ?? null);
        this.isLoading.set(false);
      },
      error: err => {
        NotificationService.showError('Could not load scrape source detail: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
        this.isLoading.set(false);
        this.router.navigate(['/scraper']).catch(() => {});
      }
    });

    this.subscriptions.add(sub);
  }

  private loadProxyList(id?: number): void {
    const sourceId = id ?? this.sourceId();
    if (!sourceId) {
      return;
    }

    this.proxyLoading.set(true);
    const sub = this.http.getScrapeSourceProxyPage(sourceId, {
      page: this.proxyPage(),
      rows: this.proxyPageSize(),
      search: this.proxySearchTerm(),
      filters: this.buildFilterPayload(this.appliedFilters()),
    }).subscribe({
      next: res => {
        this.proxies.set(res?.proxies ?? []);
        this.proxyTotal.set(res?.total ?? 0);
        this.proxyLoading.set(false);
        this.proxyHasLoaded.set(true);
      },
      error: err => {
        NotificationService.showError('Could not load scrape source proxies: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
        this.proxyLoading.set(false);
        this.proxyHasLoaded.set(true);
      }
    });

    this.subscriptions.add(sub);
  }

  private activeFilterCount(): number {
    const filters = this.appliedFilters();
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

  private ensureFilterOptionsLoaded(): void {
    if (this.filterOptionsLoaded()) {
      return;
    }

    this.http.getProxyFilterOptions().subscribe({
      next: options => {
        const normalized = this.normalizeFilterOptions(options);
        this.filterOptions.set(normalized);
        this.countryOptions.set(this.buildFilterOptionList(normalized.countries));
        this.typeOptions.set(this.buildFilterOptionList(normalized.types));
        this.anonymityOptions.set(this.buildFilterOptionList(normalized.anonymityLevels));
        this.filterOptionsLoaded.set(true);
      },
      error: err => {
        const message = err?.error?.message ?? err?.message ?? 'Unknown error';
        NotificationService.showError('Could not load filter options: ' + message);
      }
    });
  }

  private normalizeFilterOptions(options: ProxyFilterOptions): ProxyFilterOptions {
    return {
      countries: this.sortFilterOptions(options.countries),
      types: this.sortFilterOptions(options.types),
      anonymityLevels: this.sortFilterOptions(options.anonymityLevels),
    };
  }

  private sortFilterOptions(values: string[]): string[] {
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

  private buildFilterOptionList(values: string[]): FilterOption[] {
    return (values ?? []).map(value => ({label: value, value}));
  }

  private syncFilterFormWithApplied(): void {
    const filters = this.appliedFilters();
    this.filterForm.patchValue({
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
    }, {emitEvent: false});
  }

  private buildFiltersFromForm(): ProxyListAppliedFilters {
    const formValue = this.filterForm.getRawValue() as ProxyListFilterFormValues;
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
      maxTimeout: this.normalizeNumber(formValue.maxTimeout),
      maxRetries: this.normalizeNumber(formValue.maxRetries),
      countries: this.normalizeSelection(formValue.countries),
      types: this.normalizeSelection(formValue.types),
      anonymityLevels: this.normalizeSelection(formValue.anonymityLevels),
      reputationLabels: this.normalizeSelection(formValue.reputationLabels),
    };
  }

  private buildFilterPayload(filters: ProxyListAppliedFilters): ProxyListFilters | undefined {
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

  private normalizeSelection(values: string[] | null | undefined): string[] {
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

  private normalizeNumber(value: number | string | null | undefined): number {
    if (value === null || value === undefined) {
      return 0;
    }
    const parsed = typeof value === 'string' ? Number(value) : value;
    if (!Number.isFinite(parsed)) {
      return 0;
    }
    return Math.max(0, Math.floor(parsed));
  }

}
