import {
  AfterViewInit,
  Component,
  ElementRef,
  EventEmitter,
  HostListener,
  OnDestroy,
  OnInit,
  Output,
  ViewChild,
  signal
} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {CdkDragDrop, DragDropModule, moveItemInArray} from '@angular/cdk/drag-drop';
import {HttpService} from '../../services/http.service';
import {ProxyInfo} from '../../models/ProxyInfo';
import {SelectionModel} from '@angular/cdk/collections';
import {TableLazyLoadEvent} from 'primeng/table'; // Keep this for onLazyLoad
import {ButtonModule} from 'primeng/button';
import {NotificationService} from '../../services/notification-service.service';
import {Observable, Subscription} from 'rxjs';
import {filter, finalize, takeUntil} from 'rxjs/operators';
import {ExportProxiesComponent} from './export-proxies/export-proxies.component';
import {AddProxiesComponent} from './add-proxies/add-proxies.component';
import {NavigationStart, Router} from '@angular/router';
import {DeleteProxiesComponent} from './delete-proxies/delete-proxies.component';
import {InputNumberModule} from 'primeng/inputnumber';
import {MultiSelectModule} from 'primeng/multiselect';
import {ProxyListFilters} from '../../models/ProxyListFilters';
import {ProxyFilterOptions} from '../../models/ProxyFilterOptions';
import {ProxyReputation} from '../../models/ProxyReputation';
import {UserSettings} from '../../models/UserSettings';
import {ProxyFilterPanelComponent} from '../../shared/proxy-filter-panel/proxy-filter-panel.component';
import {ProxyTableComponent} from '../../shared/proxy-table/proxy-table.component';
import {
  DEFAULT_PROXY_TABLE_COLUMNS,
  PROXY_TABLE_COLUMN_DEFINITIONS,
  ProxyTableColumnDefinition,
  ProxyTableColumnId,
  getProxyTableColumnDefinition,
  normalizeProxyTableColumns,
} from '../../shared/proxy-table/proxy-table-columns';
import {SettingsService} from '../../services/settings.service';
import {
  ProxyFilterOption,
  ProxyListAppliedFilters,
  ProxyListFilterFormValues,
  PROXY_REPUTATION_OPTIONS,
  PROXY_STATUS_OPTIONS,
  activeProxyFilterCount,
  buildFilterOptionList,
  buildFiltersFromFormValue,
  buildProxyListFilterPayload,
  createDefaultProxyFilterValues,
  createDefaultProxyListAppliedFilters,
  normalizeFilterOptions,
  normalizeNumber,
  normalizeSelection,
  syncFilterFormWithApplied,
} from '../../shared/proxy-filters';

@Component({
  selector: 'app-proxy-list',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    FormsModule,
    ButtonModule,
    InputNumberModule,
    MultiSelectModule,
    DragDropModule,
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
  @ViewChild('filterToggleAnchor') private filterToggleAnchor?: ElementRef<HTMLElement>;
  @ViewChild('filterPanelRef') private filterPanelRef?: ElementRef<HTMLElement>;
  @ViewChild('columnToggleAnchor') private columnToggleAnchor?: ElementRef<HTMLElement>;
  @ViewChild('columnPanelRef') private columnPanelRef?: ElementRef<HTMLElement>;

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
  countryOptions = signal<ProxyFilterOption[]>([]);
  typeOptions = signal<ProxyFilterOption[]>([]);
  anonymityOptions = signal<ProxyFilterOption[]>([]);
  appliedFilters = signal<ProxyListAppliedFilters>(createDefaultProxyListAppliedFilters());
  displayedColumns = signal<ProxyTableColumnId[]>([...DEFAULT_PROXY_TABLE_COLUMNS]);
  columnPanelOpen = signal(false);
  columnEditorColumns = signal<ProxyTableColumnId[]>([...DEFAULT_PROXY_TABLE_COLUMNS]);
  isSavingColumnPreferences = signal(false);
  filterForm: FormGroup;
  readonly proxyStatusOptions = PROXY_STATUS_OPTIONS;
  readonly proxyReputationOptions = PROXY_REPUTATION_OPTIONS;
  readonly proxyTableColumnDefinitions = PROXY_TABLE_COLUMN_DEFINITIONS;
  private readonly defaultFilterValues: ProxyListFilterFormValues = createDefaultProxyFilterValues();
  private searchDebounceHandle?: ReturnType<typeof setTimeout>;
  private readonly pageSizeStorageKey = 'magpie-proxy-list-page-size';
  private readonly pageStorageKey = 'magpie-proxy-list-page';
  private readonly scrollStorageKey = 'magpie-proxy-list-scroll-y';
  private readonly restoreStateStorageKey = 'magpie-proxy-list-restore-state';
  private readonly filterStorageKey = 'magpie-proxy-list-filters';
  private pendingScrollY: number | null = null;
  private navigationStart$: Observable<NavigationStart>;
  private isNavigatingAway = false;

  sortField = signal<string | null>(null);
  sortOrder = signal<number | null>(null); // 1 for ascending, -1 for descending

  private proxyListSubscription?: Subscription;
  private navigationSubscription?: Subscription;
  private userSettingsSubscription?: Subscription;
  private suppressOutsideCloseUntil = 0;

  constructor(
    private http: HttpService,
    private router: Router,
    private fb: FormBuilder,
    private notification: NotificationService,
    private settingsService: SettingsService
  ) {
    this.navigationStart$ = this.router.events.pipe(
      filter((event): event is NavigationStart => event instanceof NavigationStart)
    );
    this.navigationSubscription = this.navigationStart$.subscribe(() => {
      this.isNavigatingAway = true;
      this.proxyListSubscription?.unsubscribe();
    });
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
    this.syncColumnsFromSettings(this.settingsService.getUserSettings());
    this.userSettingsSubscription = this.settingsService.userSettings$
      .pipe(filter((settings): settings is UserSettings => !!settings))
      .subscribe(settings => this.syncColumnsFromSettings(settings));

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
    this.isNavigatingAway = false;
    this.isLoading.set(true);
    const requestedRows = event?.rows ?? this.pageSize();
    const normalizedRows = Number.isFinite(requestedRows) && requestedRows > 0 ? requestedRows : this.pageSize();
    const rawPage = event ? Math.floor((event.first ?? 0) / normalizedRows) + 1 : this.page();
    const page = Number.isFinite(rawPage) && rawPage > 0 ? rawPage : 1;
    const rows = normalizedRows;
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
    })
      .pipe(
        takeUntil(this.navigationStart$),
        finalize(() => this.isLoading.set(false))
      )
      .subscribe({
        next: res => {
          if (this.isNavigatingAway) {
            return;
          }
          const data = [...res.proxies];
          this.page.set(page);
          this.pageSize.set(rows);
          const sorted = this.applySort(data, normalizedSortField, normalizedSortOrder);
          this.dataSource.set(sorted);
          this.totalItems.set(res.total ?? sorted.length);
          this.pruneSelection();
          this.hasLoaded.set(true);
          this.showAddProxiesMessage.emit(this.totalItems() === 0 && this.hasLoaded());
          this.restoreScrollPosition();
        },
        error: err => {
          if (this.isNavigatingAway) {
            return;
          }
          const message = err?.error?.message ?? err?.message ?? 'Unknown error';
          this.notification.showError('Could not get proxy page: ' + message);
          this.hasLoaded.set(true);
        }
      });
  }

  ngOnDestroy(): void {
    this.proxyListSubscription?.unsubscribe();
    this.navigationSubscription?.unsubscribe();
    this.userSettingsSubscription?.unsubscribe();
    if (this.searchDebounceHandle) {
      clearTimeout(this.searchDebounceHandle);
    }
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    if (Date.now() < this.suppressOutsideCloseUntil) {
      return;
    }

    const target = event.target as Node | null;
    if (!target) {
      return;
    }

    if (
      this.filterPanelOpen() &&
      !this.isTargetWithin(target, this.filterToggleAnchor, this.filterPanelRef)
    ) {
      this.filterPanelOpen.set(false);
    }

    if (
      this.columnPanelOpen() &&
      !this.isTargetWithin(target, this.columnToggleAnchor, this.columnPanelRef)
    ) {
      this.columnPanelOpen.set(false);
    }
  }

  onLazyLoad(event: TableLazyLoadEvent) {
    const previousSortField = this.sortField();
    const previousSortOrder = this.sortOrder();

    const requestedRows = event.rows ?? this.pageSize();
    const newPageSize = Number.isFinite(requestedRows) && requestedRows > 0 ? requestedRows : this.pageSize();
    const first = event.first ?? 0;
    const newPage = Math.floor(first / newPageSize) + 1;

    const normalizedSortOrder = event.sortOrder && event.sortOrder !== 0 ? event.sortOrder : null;
    const normalizedSortField = normalizedSortOrder ? this.resolveSortField(event.sortField) : null;

    const sortChanged = normalizedSortField !== previousSortField || normalizedSortOrder !== previousSortOrder;
    const pageChanged = newPage !== this.page();
    const pageSizeChanged = newPageSize !== this.pageSize();

    if (!Number.isFinite(newPage) || newPage < 1) {
      return;
    }

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

  toggleFilterPanel(event?: Event | { originalEvent?: Event }): void {
    this.stopTriggerEvent(event);
    const nextState = !this.filterPanelOpen();
    if (nextState) {
      this.syncFilterFormWithApplied();
      this.ensureFilterOptionsLoaded();
      this.suppressOutsideCloseUntil = Date.now() + 180;
    }
    this.filterPanelOpen.set(nextState);
  }

  openColumnPanel(event?: Event | { originalEvent?: Event }): void {
    this.stopTriggerEvent(event);
    if (this.columnPanelOpen()) {
      this.columnPanelOpen.set(false);
      return;
    }
    this.columnEditorColumns.set([...this.displayedColumns()]);
    this.suppressOutsideCloseUntil = Date.now() + 180;
    this.columnPanelOpen.set(true);
  }

  closeColumnPanel(): void {
    this.columnEditorColumns.set([...this.displayedColumns()]);
    this.columnPanelOpen.set(false);
  }

  resetColumnEditor(): void {
    this.columnEditorColumns.set([...DEFAULT_PROXY_TABLE_COLUMNS]);
  }

  onColumnDrop(event: CdkDragDrop<ProxyTableColumnDefinition[]>): void {
    if (event.previousIndex === event.currentIndex) {
      return;
    }
    const columns = [...this.columnEditorColumns()];
    moveItemInArray(columns, event.previousIndex, event.currentIndex);
    this.columnEditorColumns.set(columns);
  }

  onColumnDragStart(): void {
    this.suppressOutsideCloseUntil = Date.now() + 60_000;
  }

  onColumnDragEnd(): void {
    this.suppressOutsideCloseUntil = Date.now() + 240;
  }

  hideColumn(id: ProxyTableColumnId): void {
    const current = this.columnEditorColumns();
    if (current.length <= 1) {
      this.notification.showError('At least one column must stay visible.');
      return;
    }
    this.columnEditorColumns.set(current.filter(column => column !== id));
  }

  showColumn(id: ProxyTableColumnId): void {
    const current = this.columnEditorColumns();
    if (current.includes(id)) {
      return;
    }
    this.columnEditorColumns.set([...current, id]);
  }

  saveColumnPreferences(): void {
    const previous = this.displayedColumns();
    const next = normalizeProxyTableColumns(this.columnEditorColumns());

    this.displayedColumns.set(next);
    this.columnPanelOpen.set(false);
    this.isSavingColumnPreferences.set(true);

    this.settingsService.saveProxyListColumns(next)
      .pipe(finalize(() => this.isSavingColumnPreferences.set(false)))
      .subscribe({
        error: err => {
          this.displayedColumns.set(previous);
          const message = err?.error?.message ?? err?.message ?? 'Unknown error';
          this.notification.showError('Could not save column settings: ' + message);
        }
      });
  }

  columnPanelVisible(): ProxyTableColumnDefinition[] {
    return this.columnEditorColumns().map(column => getProxyTableColumnDefinition(column));
  }

  columnPanelHidden(): ProxyTableColumnDefinition[] {
    const selected = new Set(this.columnEditorColumns());
    return this.proxyTableColumnDefinitions.filter(column => !selected.has(column.id));
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
    this.appliedFilters.set(createDefaultProxyListAppliedFilters());
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
    return activeProxyFilterCount(this.appliedFilters());
  }

  private ensureFilterOptionsLoaded(): void {
    if (this.filterOptionsLoaded()) {
      return;
    }

    this.http.getProxyFilterOptions().subscribe({
      next: options => {
        const normalized = normalizeFilterOptions(options);
        this.filterOptions.set(normalized);
        this.countryOptions.set(buildFilterOptionList(normalized.countries));
        this.typeOptions.set(buildFilterOptionList(normalized.types));
        this.anonymityOptions.set(buildFilterOptionList(normalized.anonymityLevels));
        this.filterOptionsLoaded.set(true);
      },
      error: err => {
        const message = err?.error?.message ?? err?.message ?? 'Unknown error';
        this.notification.showError('Could not load filter options: ' + message);
      }
    });
  }

  private syncFilterFormWithApplied(): void {
    syncFilterFormWithApplied(this.filterForm, this.appliedFilters());
  }

  private buildFiltersFromForm(): ProxyListAppliedFilters {
    const formValue = this.filterForm.getRawValue() as ProxyListFilterFormValues;
    return buildFiltersFromFormValue(formValue);
  }

  private buildFilterPayload(filters: ProxyListAppliedFilters): ProxyListFilters | undefined {
    return buildProxyListFilterPayload(filters);
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
    if (field === 'ip_port') {
      const ip = proxy.ip ?? '';
      const port = Number.isFinite(proxy.port) ? proxy.port : 0;
      return `${ip}:${port.toString().padStart(5, '0')}`;
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
    const protocols = normalizeSelection(protocolsRaw.map(item => `${item}`));
    const allowedProtocols = new Set(['http', 'https', 'socks4', 'socks5']);
    const filteredProtocols = protocols.filter(protocol => allowedProtocols.has(protocol));

    const reputationRaw = Array.isArray(value['reputationLabels']) ? value['reputationLabels'] : [];
    const reputationLabels = normalizeSelection(reputationRaw.map(item => `${item}`));
    const allowedReputation = new Set(['good', 'neutral', 'poor', 'unknown']);
    const filteredReputation = reputationLabels.filter(label => allowedReputation.has(label));

    const countriesRaw = Array.isArray(value['countries']) ? value['countries'] : [];
    const typesRaw = Array.isArray(value['types']) ? value['types'] : [];
    const anonymityRaw = Array.isArray(value['anonymityLevels']) ? value['anonymityLevels'] : [];

    return {
      status,
      protocols: filteredProtocols,
      maxTimeout: normalizeNumber(value['maxTimeout'] as number | string | null | undefined),
      maxRetries: normalizeNumber(value['maxRetries'] as number | string | null | undefined),
      countries: normalizeSelection(countriesRaw.map(item => `${item}`)),
      types: normalizeSelection(typesRaw.map(item => `${item}`)),
      anonymityLevels: normalizeSelection(anonymityRaw.map(item => `${item}`)),
      reputationLabels: filteredReputation,
    };
  }

  private syncColumnsFromSettings(settings: UserSettings | undefined): void {
    const normalized = normalizeProxyTableColumns(settings?.proxy_list_columns ?? DEFAULT_PROXY_TABLE_COLUMNS);
    this.displayedColumns.set(normalized);
    if (!this.columnPanelOpen()) {
      this.columnEditorColumns.set([...normalized]);
    }
  }

  private isTargetWithin(target: Node, ...elements: Array<ElementRef<HTMLElement> | undefined>): boolean {
    for (const elementRef of elements) {
      const element = elementRef?.nativeElement;
      if (element && element.contains(target)) {
        return true;
      }
    }
    return false;
  }

  private stopTriggerEvent(event?: Event | { originalEvent?: Event }): void {
    if (!event) {
      return;
    }
    const domEvent = (event as { originalEvent?: Event }).originalEvent ?? (event as Event);
    domEvent?.stopPropagation?.();
  }
}
