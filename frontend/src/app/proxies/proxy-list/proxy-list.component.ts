import {AfterViewInit, Component, EventEmitter, OnDestroy, OnInit, Output, signal} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {HttpService} from '../../services/http.service';
import {ProxyInfo} from '../../models/ProxyInfo';
import {SelectionModel} from '@angular/cdk/collections';
import {TableLazyLoadEvent} from 'primeng/table'; // Keep this for onLazyLoad
import {ButtonModule} from 'primeng/button';
import {NotificationService} from '../../services/notification-service.service';
import {Subscription} from 'rxjs';
import {ExportProxiesComponent} from './export-proxies/export-proxies.component';
import {AddProxiesComponent} from './add-proxies/add-proxies.component';
import {Router} from '@angular/router';
import {DeleteProxiesComponent} from './delete-proxies/delete-proxies.component';
import {InputNumberModule} from 'primeng/inputnumber';
import {MultiSelectModule} from 'primeng/multiselect';
import {ProxyListFilters} from '../../models/ProxyListFilters';
import {ProxyFilterOptions} from '../../models/ProxyFilterOptions';
import {ProxyReputation} from '../../models/ProxyReputation';
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

@Component({
  selector: 'app-proxy-list',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    FormsModule,
    ButtonModule,
    InputNumberModule,
    MultiSelectModule,
    AddProxiesComponent,
    ExportProxiesComponent,
    DeleteProxiesComponent,
    ProxyFilterPanelComponent,
    ProxyTableComponent,
  ],
  templateUrl: './proxy-list.component.html',
  styleUrls: ['./proxy-list.component.scss']
})
export class ProxyListComponent implements OnInit, AfterViewInit, OnDestroy {
  @Output() showAddProxiesMessage = new EventEmitter<boolean>();

  dataSource = signal<ProxyInfo[]>([]);
  selection = new SelectionModel<ProxyInfo>(true, []);
  selectedProxies = signal<ProxyInfo[]>([]);
  page = signal(1);
  pageSize = signal(40);
  readonly rowsPerPageOptions = [20, 40, 60, 100];
  readonly skeletonRows = Array.from({ length: 8 });
  totalItems = signal(0);
  hasLoaded = signal(false);
  isLoading = signal(false);
  searchTerm = signal('');
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
  private searchDebounceHandle?: ReturnType<typeof setTimeout>;
  private readonly pageSizeStorageKey = 'magpie-proxy-list-page-size';
  private readonly pageStorageKey = 'magpie-proxy-list-page';
  private readonly scrollStorageKey = 'magpie-proxy-list-scroll-y';
  private readonly restoreStateStorageKey = 'magpie-proxy-list-restore-state';
  private readonly filterStorageKey = 'magpie-proxy-list-filters';
  private pendingScrollY: number | null = null;

  sortField = signal<string | null>(null);
  sortOrder = signal<number | null>(null); // 1 for ascending, -1 for descending

  private proxyListSubscription?: Subscription;

  constructor(private http: HttpService, private router: Router, private fb: FormBuilder) {
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

  ngAfterViewInit() {
    // PrimeNG table handles sorting internally with pSortableColumn and (onSort)
  }

  ngOnInit(): void {
    const storedPageSize = this.getStoredPageSize();
    if (storedPageSize !== null) {
      this.pageSize.set(storedPageSize);
    }
    const shouldRestoreState = this.consumeRestoreState();
    if (shouldRestoreState) {
      const storedPage = this.getStoredPage();
      if (storedPage !== null) {
        this.page.set(storedPage);
      }
      this.pendingScrollY = this.getStoredScrollY();
      this.clearStoredPageAndScroll();
    } else {
      this.page.set(1);
      this.pendingScrollY = null;
      this.clearStoredPageAndScroll();
    }
    const storedFilters = this.getStoredFilters();
    if (storedFilters) {
      this.appliedFilters.set(storedFilters);
    }
    this.getAndSetProxyList();
  }

  getAndSetProxyList(event?: TableLazyLoadEvent) {
    this.proxyListSubscription?.unsubscribe();
    this.isLoading.set(true);
    const page = event ? Math.floor((event.first ?? 0) / (event.rows ?? this.pageSize())) + 1 : this.page();
    const rows = event?.rows ?? this.pageSize();
    const requestedSortField = this.resolveSortField(event?.sortField);
    const requestedSortOrder = event?.sortOrder ?? this.sortOrder() ?? null;
    const normalizedSortOrder = requestedSortOrder && requestedSortOrder !== 0 ? requestedSortOrder : null;
    const normalizedSortField = normalizedSortOrder ? requestedSortField : null;

    this.sortField.set(normalizedSortField);
    this.sortOrder.set(normalizedSortOrder);

    const trimmedSearch = this.searchTerm().trim();
    const filterPayload = this.buildFilterPayload(this.appliedFilters());

    this.proxyListSubscription = this.http.getProxyPage(page, {
      rows,
      search: trimmedSearch.length > 0 ? trimmedSearch : undefined,
      filters: filterPayload,
    }).subscribe({
      next: res => {
        const data = [...res.proxies];
        this.page.set(page);
        this.pageSize.set(rows);
        const sorted = this.applySort(data, normalizedSortField, normalizedSortOrder);
        this.dataSource.set(sorted);
        this.totalItems.set(res.total ?? sorted.length);
        this.pruneSelection();
        this.isLoading.set(false);
        this.hasLoaded.set(true);
        this.showAddProxiesMessage.emit(this.totalItems() === 0 && this.hasLoaded());
        this.restoreScrollPosition();
      },
      error: err => {
        NotificationService.showError('Could not get proxy page: ' + err.error.message);
        this.isLoading.set(false);
        this.hasLoaded.set(true);
      }
    });
  }

  ngOnDestroy(): void {
    this.proxyListSubscription?.unsubscribe();
    if (this.searchDebounceHandle) {
      clearTimeout(this.searchDebounceHandle);
    }
  }

  onLazyLoad(event: TableLazyLoadEvent) {
    const previousSortField = this.sortField();
    const previousSortOrder = this.sortOrder();

    const newPage = Math.floor(event.first! / event.rows!) + 1;
    const newPageSize = event.rows ?? this.pageSize();

    const normalizedSortOrder = event.sortOrder && event.sortOrder !== 0 ? event.sortOrder : null;
    const normalizedSortField = normalizedSortOrder ? this.resolveSortField(event.sortField) : null;

    const sortChanged = normalizedSortField !== previousSortField || normalizedSortOrder !== previousSortOrder;
    const pageChanged = newPage !== this.page();
    const pageSizeChanged = newPageSize !== this.pageSize();

    this.page.set(newPage);
    this.pageSize.set(newPageSize);
    this.sortField.set(normalizedSortField);
    this.sortOrder.set(normalizedSortOrder);

    if (pageSizeChanged) {
      this.persistPageSize(newPageSize);
    }

    if (!sortChanged && (pageChanged || pageSizeChanged)) {
      this.getAndSetProxyList(event);
    }
  }

  onSort(event: { field: string; order: number }) {
    const hasOrder = event.order !== 0 && event.order !== undefined && event.order !== null;
    this.sortField.set(hasOrder ? this.resolveSortField(event.field) : null);
    this.sortOrder.set(hasOrder ? event.order : null);
    const sorted = this.applySort([...this.dataSource()], this.sortField(), this.sortOrder());
    this.dataSource.set(sorted);
  }

  toggleSelection(proxy: ProxyInfo): void {
    this.selection.toggle(proxy);
    this.selectedProxies.set([...this.selection.selected]);
  }

  isAllSelected(): boolean {
    const numSelected = this.selection.selected.length;
    const numRows = this.dataSource().length;
    return numSelected === numRows && numRows > 0; // Added numRows > 0 to handle empty table case
  }

  masterToggle(): void {
    this.isAllSelected() ?
      this.selection.clear() :
      this.dataSource().forEach(row => this.selection.select(row));
    this.selectedProxies.set([...this.selection.selected]);
  }

  onProxiesDeleted(): void {
    this.selection.clear();
    this.selectedProxies.set([]);
    this.getAndSetProxyList();
  }

  onSearchTermChange(value: string): void {
    if (this.searchDebounceHandle) {
      clearTimeout(this.searchDebounceHandle);
    }

    this.searchTerm.set(value);
    this.searchDebounceHandle = setTimeout(() => {
      this.page.set(1);
      this.getAndSetProxyList();
    }, 300);
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
    this.persistFilters(nextFilters);
    this.page.set(1);
    this.getAndSetProxyList();
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
    this.clearStoredFilters();
    this.page.set(1);
    this.getAndSetProxyList();
  }

  hasActiveFilters(): boolean {
    return this.activeFilterCount() > 0;
  }

  filterButtonLabel(): string {
    const count = this.activeFilterCount();
    if (count === 0) {
      return 'Filters';
    }
    return `Filters (${count})`;
  }

  filterToggleClass(): string {
    if (this.hasActiveFilters()) {
      return 'p-button-outlined filter-toggle filter-toggle--active';
    }
    return 'p-button-outlined filter-toggle';
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

  private resolveSortField(sortField: TableLazyLoadEvent['sortField']): string | null {
    if (!sortField) {
      return this.sortField() ?? null;
    }

    return Array.isArray(sortField) ? sortField[0] : sortField;
  }

  private applySort(data: ProxyInfo[], sortField: string | null | undefined, sortOrder: number | null | undefined): ProxyInfo[] {
    if (!sortField || !sortOrder || sortOrder === 0) {
      return data;
    }

    const direction = sortOrder === 1 ? 1 : -1;

    return data.sort((a, b) => {
      const valueA = this.normalizeSortableValue(this.getSortableValue(a, sortField));
      const valueB = this.normalizeSortableValue(this.getSortableValue(b, sortField));

      if (valueA === valueB) {
        return 0;
      }

      if (valueA === undefined || valueA === null) {
        return 1 * direction;
      }

      if (valueB === undefined || valueB === null) {
        return -1 * direction;
      }

      if (valueA < valueB) {
        return -1 * direction;
      }

      if (valueA > valueB) {
        return 1 * direction;
      }

      return 0;
    });
  }

  private normalizeSortableValue(value: unknown): string | number | null {
    if (value === null || value === undefined) {
      return null;
    }

    if (typeof value === 'number') {
      return value;
    }

    if (typeof value === 'boolean') {
      return value ? 1 : 0;
    }

    if (value instanceof Date) {
      return value.getTime();
    }

    if (typeof value === 'string') {
      const timestamp = Date.parse(value);
      return Number.isNaN(timestamp) ? value.toLowerCase() : timestamp;
    }

    return null;
  }

  private getSortableValue(proxy: ProxyInfo, field: string | null | undefined): unknown {
    if (!field) {
      return null;
    }

    if (field === 'reputation') {
      return this.getPrimaryReputation(proxy)?.score ?? null;
    }

    if (Object.prototype.hasOwnProperty.call(proxy, field)) {
      return proxy[field as keyof ProxyInfo];
    }

    return null;
  }

  private getPrimaryReputation(proxy: ProxyInfo): ProxyReputation | null {
    const reputation = proxy.reputation;
    if (!reputation) {
      return null;
    }

    if (reputation.overall) {
      return reputation.overall;
    }

    if (reputation.protocols) {
      for (const rep of Object.values(reputation.protocols)) {
        if (rep) {
          return rep;
        }
      }
    }

    return null;
  }

  onProxiesAdded(): void {
    this.selection.clear();
    this.selectedProxies.set([]);
    this.page.set(1);
    this.getAndSetProxyList();
  }

  refreshList(): void {
    if (this.searchDebounceHandle) {
      clearTimeout(this.searchDebounceHandle);
      this.searchDebounceHandle = undefined;
    }

    this.getAndSetProxyList();
  }

  private pruneSelection(): void {
    if (this.selection.isEmpty()) {
      this.selectedProxies.set([]);
      return;
    }

    const ids = new Set(this.dataSource().map(proxy => proxy.id));
    const retained = this.selection.selected.filter(proxy => ids.has(proxy.id));

    this.selection.clear();
    retained.forEach(proxy => this.selection.select(proxy));
    this.selectedProxies.set([...retained]);
  }

  onViewProxy(proxy: ProxyInfo): void {
    this.markRestoreState();
    this.persistPage(this.page());
    this.persistScrollPosition();
    this.router.navigate(['/proxies', proxy.id]).catch(() => {});
  }

  onSelectionChange(selected: ProxyInfo[]): void {
    this.selection.clear();
    selected.forEach(proxy => this.selection.select(proxy));
    this.selectedProxies.set([...selected]);
  }

  private getStoredPageSize(): number | null {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return null;
      }
      const raw = storage.getItem(this.pageSizeStorageKey);
      if (!raw) {
        return null;
      }
      const parsed = Number(raw);
      if (!Number.isFinite(parsed) || parsed <= 0) {
        return null;
      }
      if (!this.rowsPerPageOptions.includes(parsed)) {
        return null;
      }
      return parsed;
    } catch {
      return null;
    }
  }

  private persistPageSize(size: number): void {
    if (!Number.isFinite(size) || size <= 0) {
      return;
    }
    try {
      const storage = this.getStorage();
      if (!storage) {
        return;
      }
      storage.setItem(this.pageSizeStorageKey, size.toString());
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private getStoredPage(): number | null {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return null;
      }
      const raw = storage.getItem(this.pageStorageKey);
      if (!raw) {
        return null;
      }
      const parsed = Number(raw);
      if (!Number.isFinite(parsed) || parsed < 1) {
        return null;
      }
      return Math.floor(parsed);
    } catch {
      return null;
    }
  }

  private persistPage(page: number): void {
    if (!Number.isFinite(page) || page < 1) {
      return;
    }
    try {
      const storage = this.getStorage();
      if (!storage) {
        return;
      }
      storage.setItem(this.pageStorageKey, Math.floor(page).toString());
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private clearStoredPageAndScroll(): void {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return;
      }
      storage.removeItem(this.pageStorageKey);
      storage.removeItem(this.scrollStorageKey);
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private markRestoreState(): void {
    try {
      const storage = this.getSessionStorage();
      if (!storage) {
        return;
      }
      storage.setItem(this.restoreStateStorageKey, '1');
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private consumeRestoreState(): boolean {
    try {
      const storage = this.getSessionStorage();
      if (!storage) {
        return false;
      }
      const raw = storage.getItem(this.restoreStateStorageKey);
      if (raw !== '1') {
        return false;
      }
      storage.removeItem(this.restoreStateStorageKey);
      return true;
    } catch {
      return false;
    }
  }

  private getStoredScrollY(): number | null {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return null;
      }
      const raw = storage.getItem(this.scrollStorageKey);
      if (raw === null) {
        return null;
      }
      const parsed = Number(raw);
      if (!Number.isFinite(parsed) || parsed < 0) {
        return null;
      }
      return parsed;
    } catch {
      return null;
    }
  }

  private persistScrollPosition(): void {
    if (typeof window === 'undefined') {
      return;
    }
    const container = this.getScrollContainer();
    const scrollY = container ? container.scrollTop : (window.scrollY ?? window.pageYOffset ?? 0);
    if (!Number.isFinite(scrollY) || scrollY < 0) {
      return;
    }
    try {
      const storage = this.getStorage();
      if (!storage) {
        return;
      }
      storage.setItem(this.scrollStorageKey, Math.floor(scrollY).toString());
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private restoreScrollPosition(): void {
    if (this.pendingScrollY === null || typeof window === 'undefined') {
      return;
    }
    const target = this.pendingScrollY;
    this.pendingScrollY = null;
    const container = this.getScrollContainer();
    setTimeout(() => {
      if (container) {
        container.scrollTop = target;
      } else {
        window.scrollTo({ top: target, left: 0, behavior: 'auto' });
      }
    }, 0);
  }

  private getScrollContainer(): HTMLElement | null {
    if (typeof document === 'undefined') {
      return null;
    }
    return document.querySelector('main');
  }

  private getSessionStorage(): Storage | null {
    if (typeof window === 'undefined' || !window?.sessionStorage) {
      return null;
    }
    return window.sessionStorage;
  }

  private getStorage(): Storage | null {
    if (typeof window === 'undefined' || !window?.localStorage) {
      return null;
    }
    return window.localStorage;
  }

  private getStoredFilters(): ProxyListAppliedFilters | null {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return null;
      }
      const raw = storage.getItem(this.filterStorageKey);
      if (!raw) {
        return null;
      }
      const parsed = JSON.parse(raw) as Record<string, unknown>;
      return this.normalizeStoredFilters(parsed);
    } catch {
      return null;
    }
  }

  private persistFilters(filters: ProxyListAppliedFilters): void {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return;
      }
      storage.setItem(this.filterStorageKey, JSON.stringify(filters));
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private clearStoredFilters(): void {
    try {
      const storage = this.getStorage();
      if (!storage) {
        return;
      }
      storage.removeItem(this.filterStorageKey);
    } catch {
      // ignore persistence errors (private browsing, SSR)
    }
  }

  private normalizeStoredFilters(value: Record<string, unknown> | null): ProxyListAppliedFilters | null {
    if (!value) {
      return null;
    }

    const statusValue = `${value['status'] ?? ''}`;
    const status: ProxyListAppliedFilters['status'] = statusValue === 'alive' || statusValue === 'dead' ? statusValue : 'all';
    const protocolsRaw = Array.isArray(value['protocols']) ? value['protocols'] : [];
    const protocols = this.normalizeSelection(protocolsRaw.map(item => `${item}`));
    const allowedProtocols = new Set(['http', 'https', 'socks4', 'socks5']);
    const filteredProtocols = protocols.filter(protocol => allowedProtocols.has(protocol));

    const reputationRaw = Array.isArray(value['reputationLabels']) ? value['reputationLabels'] : [];
    const reputationLabels = this.normalizeSelection(reputationRaw.map(item => `${item}`));
    const allowedReputation = new Set(['good', 'neutral', 'poor', 'unknown']);
    const filteredReputation = reputationLabels.filter(label => allowedReputation.has(label));

    const countriesRaw = Array.isArray(value['countries']) ? value['countries'] : [];
    const typesRaw = Array.isArray(value['types']) ? value['types'] : [];
    const anonymityRaw = Array.isArray(value['anonymityLevels']) ? value['anonymityLevels'] : [];

    return {
      status,
      protocols: filteredProtocols,
      maxTimeout: this.normalizeNumber(value['maxTimeout'] as number | string | null | undefined),
      maxRetries: this.normalizeNumber(value['maxRetries'] as number | string | null | undefined),
      countries: this.normalizeSelection(countriesRaw.map(item => `${item}`)),
      types: this.normalizeSelection(typesRaw.map(item => `${item}`)),
      anonymityLevels: this.normalizeSelection(anonymityRaw.map(item => `${item}`)),
      reputationLabels: filteredReputation,
    };
  }
}
