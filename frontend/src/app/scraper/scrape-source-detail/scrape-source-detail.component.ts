import {CdkDragDrop, DragDropModule, moveItemInArray} from '@angular/cdk/drag-drop';
import {Component, ElementRef, HostListener, OnDestroy, OnInit, ViewChild, signal} from '@angular/core';
import { CommonModule, DatePipe, NgClass } from '@angular/common';
import {FormBuilder, FormGroup} from '@angular/forms';
import {ActivatedRoute, Router, RouterLink} from '@angular/router';
import {Subscription} from 'rxjs';
import {ClipboardModule, Clipboard} from '@angular/cdk/clipboard';
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
import {
  DEFAULT_PROXY_TABLE_COLUMNS,
  PROXY_TABLE_COLUMN_DEFINITIONS,
  ProxyTableColumnDefinition,
  ProxyTableColumnId,
  getProxyTableColumnDefinition,
  normalizeProxyTableColumns,
} from '../../shared/proxy-table/proxy-table-columns';
import {SettingsService} from '../../services/settings.service';
import {UserSettings} from '../../models/UserSettings';
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
  syncFilterFormWithApplied,
} from '../../shared/proxy-filters';
import {filter, finalize} from 'rxjs/operators';

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
    ClipboardModule,
    DragDropModule,
    ProxyFilterPanelComponent,
    ProxyTableComponent,
  ],
  templateUrl: './scrape-source-detail.component.html',
  styleUrl: './scrape-source-detail.component.scss'
})
export class ScrapeSourceDetailComponent implements OnInit, OnDestroy {
  @ViewChild('filterToggleAnchor') private filterToggleAnchor?: ElementRef<HTMLElement>;
  @ViewChild('filterPanelRef') private filterPanelRef?: ElementRef<HTMLElement>;
  @ViewChild('columnToggleAnchor') private columnToggleAnchor?: ElementRef<HTMLElement>;
  @ViewChild('columnPanelRef') private columnPanelRef?: ElementRef<HTMLElement>;

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
  readonly proxySkeletonRows = Array.from({ length: 6 });
  private suppressOutsideCloseUntil = 0;

  private proxySearchDebounce?: ReturnType<typeof setTimeout>;

  private subscriptions = new Subscription();

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private http: HttpService,
    private fb: FormBuilder,
    private clipboard: Clipboard,
    private notification: NotificationService,
    private settingsService: SettingsService,
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
    this.syncColumnsFromSettings(this.settingsService.getUserSettings());
    const settingsSub = this.settingsService.userSettings$
      .pipe(filter((settings): settings is UserSettings => !!settings))
      .subscribe(settings => this.syncColumnsFromSettings(settings));
    this.subscriptions.add(settingsSub);

    const sub = this.route.paramMap.subscribe(params => {
      const rawId = params.get('id');
      const id = rawId ? Number(rawId) : NaN;
      if (!Number.isFinite(id) || id <= 0) {
        this.notification.showError('Invalid scrape source identifier');
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
      this.columnPanelOpen.set(false);
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

    this.settingsService.saveScrapeSourceProxyColumns(next)
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
    this.proxyPage.set(1);
    this.loadProxyList();
    this.filterPanelOpen.set(false);
  }

  clearFilters(): void {
    this.filterForm.reset(this.defaultFilterValues);
    this.appliedFilters.set(createDefaultProxyListAppliedFilters());
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

  copyUrl(): void {
    const value = this.detail()?.url?.trim();
    if (!value) {
      return;
    }
    this.copyToClipboard(value, 'URL copied');
  }

  private loadScrapeSourceDetail(id: number): void {
    this.isLoading.set(true);
    const sub = this.http.getScrapeSourceDetail(id).subscribe({
      next: detail => {
        this.detail.set(detail ?? null);
        this.isLoading.set(false);
      },
      error: err => {
        this.notification.showError('Could not load scrape source detail: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
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
        this.notification.showError('Could not load scrape source proxies: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
        this.proxyLoading.set(false);
        this.proxyHasLoaded.set(true);
      }
    });

    this.subscriptions.add(sub);
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

  private copyToClipboard(value: string, successMessage: string): void {
    const copied = this.clipboard.copy(value);
    if (copied) {
      this.notification.showSuccess(successMessage);
      return;
    }

    if (navigator?.clipboard?.writeText) {
      navigator.clipboard.writeText(value).then(
        () => this.notification.showSuccess(successMessage),
        () => this.notification.showError('Failed to access clipboard')
      );
      return;
    }

    this.notification.showError('Clipboard not available');
  }

  private buildFilterPayload(filters: ProxyListAppliedFilters): ProxyListFilters | undefined {
    return buildProxyListFilterPayload(filters);
  }

  private syncColumnsFromSettings(settings: UserSettings | undefined): void {
    const normalized = normalizeProxyTableColumns(settings?.scrape_source_proxy_columns ?? DEFAULT_PROXY_TABLE_COLUMNS);
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
