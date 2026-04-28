import {Component, ElementRef, EventEmitter, HostListener, OnDestroy, OnInit, Output, ViewChild, signal} from '@angular/core';
import {CommonModule} from '@angular/common';
import {FormsModule} from '@angular/forms';
import {SelectionModel} from '@angular/cdk/collections';
import {Router} from '@angular/router';
import {HttpService} from '../../services/http.service';
import {ScrapeSourceInfo} from '../../models/ScrapeSourceInfo';
import {AddScrapeSourceComponent} from '../add-scrape-source/add-scrape-source.component';
import {ExportSourcesComponent} from './export-sources/export-sources.component';
import {DeleteSourcesComponent} from './delete-sources/delete-sources.component';

// PrimeNG imports
import {TableLazyLoadEvent, TableModule} from 'primeng/table';
import {ButtonModule} from 'primeng/button';
import {CheckboxModule} from 'primeng/checkbox';
import {SkeletonModule} from 'primeng/skeleton';
import {NotificationService} from '../../services/notification-service.service';
import {SettingsService} from '../../services/settings.service';
import {UserSettings} from '../../models/UserSettings';
import {UserService} from '../../services/authorization/user.service';
import {
  DEFAULT_SCRAPE_SOURCE_LIST_COLUMNS,
  getScrapeSourceListColumnDefinition,
  normalizeScrapeSourceListColumns,
  SCRAPE_SOURCE_LIST_COLUMN_DEFINITIONS,
  ScrapeSourceListColumnDefinition,
  ScrapeSourceListColumnId
} from './scrape-source-list-columns';
import {filter, finalize} from 'rxjs/operators';
import {Subscription} from 'rxjs';
import {HealthBarCellComponent} from '../../shared/health-bar-cell/health-bar-cell.component';
import {ColumnPickerPanelComponent} from '../../shared/column-picker-panel/column-picker-panel.component';

interface ScrapeSourceView extends ScrapeSourceInfo {
  urlHead: string;
  urlTail: string;
}

@Component({
  selector: 'app-scrape-source-list',
  imports: [
    CommonModule,
    FormsModule,
    TableModule,
    ButtonModule,
    CheckboxModule,
    SkeletonModule,
    AddScrapeSourceComponent,
    ExportSourcesComponent,
    DeleteSourcesComponent,
    HealthBarCellComponent,
    ColumnPickerPanelComponent,
  ],
  templateUrl: './scrape-source-list.component.html',
  styleUrl: './scrape-source-list.component.scss',
  standalone: true
})
export class ScrapeSourceListComponent implements OnInit, OnDestroy {
  @Output() showAddScrapeSourceMessage = new EventEmitter<boolean>();
  @ViewChild('columnToggleAnchor') private columnToggleAnchor?: ElementRef<HTMLElement>;
  @ViewChild('columnPanelRef', { read: ElementRef }) private columnPanelRef?: ElementRef<HTMLElement>;

  scrapeSources: ScrapeSourceView[] = [];
  selection = new SelectionModel<ScrapeSourceView>(true, []);
  selectedScrapeSources: ScrapeSourceView[] = [];
  page = 0; // PrimeNG uses 0-based pagination
  pageSize = 20;
  totalItems = 0;
  hasLoaded = false;
  loading = false;
  checkingRobots: Record<number, boolean> = {};
  scrapingSources: Record<number, boolean> = {};
  respectRobotsEnabled = false;
  columnPanelOpen = false;
  sortField: string | null = null;
  sortOrder: number | null = null;
  isAdmin = UserService.isAdmin();
  isSavingColumnPreferences = signal(false);
  displayedColumns: ScrapeSourceListColumnId[] = [...DEFAULT_SCRAPE_SOURCE_LIST_COLUMNS];
  readonly skeletonRows = Array.from({ length: 6 });
  readonly defaultScrapeSourceColumns = DEFAULT_SCRAPE_SOURCE_LIST_COLUMNS;
  readonly scrapeSourceColumnDefinitions = SCRAPE_SOURCE_LIST_COLUMN_DEFINITIONS;
  columnPickerColumns: readonly ScrapeSourceListColumnDefinition[] = this.resolveColumnPickerColumns();

  private subscriptions = new Subscription();
  private suppressOutsideCloseUntil = 0;

  constructor(
    private http: HttpService,
    private router: Router,
    private notification: NotificationService,
    private settingsService: SettingsService,
    private userService: UserService,
  ) { }

  ngOnInit(): void {
    this.columnPickerColumns = this.resolveColumnPickerColumns();
    this.syncColumnsFromSettings(this.settingsService.getUserSettings());
    const settingsSub = this.settingsService.userSettings$
      .pipe(filter((settings): settings is UserSettings => !!settings))
      .subscribe(settings => this.syncColumnsFromSettings(settings));
    const roleSub = this.userService.role$.subscribe(role => {
      if (role === undefined) {
        return;
      }

      this.isAdmin = role === 'admin';
      this.columnPickerColumns = this.resolveColumnPickerColumns();
      this.syncColumnsFromSettings(this.settingsService.getUserSettings());
      if (!this.isAdmin) {
        this.scrapingSources = {};
      }
    });
    this.subscriptions.add(settingsSub);
    this.subscriptions.add(roleSub);

    this.loadRespectRobotsSetting();
    this.getAndSetScrapeSourceCount();
    this.getAndSetScrapeSourcesList();
  }

  ngOnDestroy(): void {
    this.subscriptions.unsubscribe();
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    if (Date.now() < this.suppressOutsideCloseUntil) {
      return;
    }

    if (!this.columnPanelOpen) {
      return;
    }

    const target = event.target as Node | null;
    if (!target) {
      return;
    }

    if (!this.isTargetWithin(target, this.columnToggleAnchor, this.columnPanelRef)) {
      this.columnPanelOpen = false;
    }
  }

  private loadRespectRobotsSetting(): void {
    this.http.getRespectRobotsSetting().subscribe({
      next: res => {
        this.respectRobotsEnabled = !!res?.respect_robots_txt;
      },
      error: err => {
        this.notification.showWarn('Could not load robots.txt setting: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
        this.respectRobotsEnabled = false;
      }
    });
  }

  getAndSetScrapeSourcesList() {
    this.loading = true;
    this.http.getScrapingSourcePage(this.page + 1).subscribe({
      next: res => {
        const sources = Array.isArray(res) ? res : [];
        this.scrapeSources = this.applySort(
          sources.map(source => this.buildViewSource(source)),
          this.sortField,
          this.sortOrder
        );
        this.syncSelectionWithData();
        this.loading = false;
        this.hasLoaded = true;
        const shouldShowEmptyState = this.totalItems === 0 && sources.length === 0;
        this.showAddScrapeSourceMessage.emit(shouldShowEmptyState);
      },
      error: err => {
        this.notification.showError("Could not get scraping sources" + err.error.message);
        this.loading = false;
        this.hasLoaded = true;
        this.showAddScrapeSourceMessage.emit(false);
      }
    });
  }

  getAndSetScrapeSourceCount() {
    this.http.getScrapingSourcesCount().subscribe({
      next: res => {
        this.totalItems = res ?? 0;
        const shouldShowEmptyState = this.totalItems === 0 && this.hasLoaded && this.scrapeSources.length === 0;
        this.showAddScrapeSourceMessage.emit(shouldShowEmptyState);
      },
      error: err => {
        this.notification.showError("Could not get scrape sources count " + err.error.message);
      }
    });
  }

  onLazyLoad(event: TableLazyLoadEvent) {
    const newPage = Math.floor((event.first ?? 0) / (event.rows ?? this.pageSize));
    const newPageSize = event.rows ?? this.pageSize;
    const nextSortOrder = event.sortOrder && event.sortOrder !== 0 ? event.sortOrder : null;
    const nextSortField = nextSortOrder ? this.resolveSortField(event.sortField) : null;

    const sortChanged = nextSortField !== this.sortField || nextSortOrder !== this.sortOrder;
    const shouldFetch = newPage !== this.page || newPageSize !== this.pageSize;

    this.page = newPage;
    this.pageSize = newPageSize;
    this.sortField = nextSortField;
    this.sortOrder = nextSortOrder;

    if (sortChanged) {
      this.scrapeSources = this.applySort([...this.scrapeSources], this.sortField, this.sortOrder);
      this.syncSelectionWithData();
    }

    if (shouldFetch) {
      this.getAndSetScrapeSourcesList();
    }
  }

  // Helper method to get selection count
  getSelectionCount(): number {
    return this.selection.selected.length;
  }

  toggleSelection(source: ScrapeSourceView): void {
    this.selection.toggle(source);
    this.selectedScrapeSources = [...this.selection.selected];
  }

  isAllSelected(): boolean {
    return this.scrapeSources.length > 0 && this.selection.selected.length === this.scrapeSources.length;
  }

  isSomeSelected(): boolean {
    const count = this.selection.selected.length;
    return count > 0 && count < this.scrapeSources.length;
  }

  masterToggle(): void {
    if (this.isAllSelected()) {
      this.selection.clear();
    } else {
      this.scrapeSources.forEach(source => this.selection.select(source));
    }
    this.selectedScrapeSources = [...this.selection.selected];
  }

  refreshList(): void {
    this.selection.clear();
    this.selectedScrapeSources = [];
    this.getAndSetScrapeSourceCount();
    this.getAndSetScrapeSourcesList();
  }

  openColumnPanel(event?: Event | { originalEvent?: Event }): void {
    this.stopTriggerEvent(event);
    if (this.columnPanelOpen) {
      this.columnPanelOpen = false;
      return;
    }
    this.suppressOutsideCloseUntil = Date.now() + 180;
    this.columnPanelOpen = true;
  }

  closeColumnPanel(): void {
    this.columnPanelOpen = false;
  }

  onColumnEditorDragStart(): void {
    this.suppressOutsideCloseUntil = Date.now() + 60_000;
  }

  onColumnEditorDragEnd(): void {
    this.suppressOutsideCloseUntil = Date.now() + 240;
  }

  saveColumnPreferences(nextColumns: string[]): void {
    const previous = [...this.displayedColumns];
    const next = normalizeScrapeSourceListColumns(nextColumns);

    this.displayedColumns = next;
    this.columnPanelOpen = false;
    this.isSavingColumnPreferences.set(true);

    this.settingsService.saveScrapeSourceListColumns(next)
      .pipe(finalize(() => { this.isSavingColumnPreferences.set(false); }))
      .subscribe({
        error: err => {
          this.displayedColumns = previous;
          const message = err?.error?.message ?? err?.message ?? 'Unknown error';
          this.notification.showError('Could not save column settings: ' + message);
        }
      });
  }

  tableColumns(): ScrapeSourceListColumnDefinition[] {
    return this.displayedColumns
      .map(column => getScrapeSourceListColumnDefinition(column))
      .filter(column => this.isAdmin || column.id !== 'scrape_now')
      .filter(column => this.respectRobotsEnabled || column.id !== 'robots_check');
  }

  tableColumnCount(): number {
    return this.tableColumns().length + 1;
  }

  trackByColumn(_index: number, column: ScrapeSourceListColumnDefinition): ScrapeSourceListColumnId {
    return column.id;
  }

  onScrapeSourcesAdded(): void {
    this.page = 0;
    this.refreshList();
  }

  onScrapeSourcesDeleted(): void {
    this.page = 0;
    this.refreshList();
  }

  onShowAddScrapeSourcesMessage(value: boolean): void {
    this.showAddScrapeSourceMessage.emit(value);
  }

  onViewSource(event: Event | { originalEvent?: Event }, source: ScrapeSourceView): void {
    if ((event as { originalEvent?: Event }).originalEvent) {
      (event as { originalEvent?: Event }).originalEvent?.stopPropagation?.();
    } else {
      (event as Event)?.stopPropagation?.();
    }
    this.router.navigate(['/scraper', source.id]).catch(() => {});
  }

  checkRobots(source: ScrapeSourceView, event?: Event): void {
    event?.stopPropagation();
    if (!source?.url) {
      return;
    }

    this.checkingRobots[source.id] = true;
    this.http.checkScrapeSource(source.url).subscribe({
      next: res => {
        const { allowed, robots_found, error } = res ?? { allowed: true, robots_found: false };

        if (allowed && robots_found) {
          this.notification.showSuccess('robots.txt allows scraping this URL');
        } else if (!allowed && robots_found) {
          this.notification.showWarn('robots.txt disallows scraping this URL');
        } else if (allowed && !robots_found) {
          this.notification.showInfo('No robots.txt found; scraping is allowed by default');
        }

        if (error) {
          this.notification.showWarn('Robots check completed with warnings: ' + error);
        }
      },
      error: err => {
        this.notification.showError('Could not check robots.txt: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
      }
    }).add(() => {
      delete this.checkingRobots[source.id];
    });
  }

  isCheckingRobots(sourceId: number): boolean {
    return this.checkingRobots[sourceId];
  }

  isScrapingSource(sourceId: number): boolean {
    return this.scrapingSources[sourceId];
  }

  scrapeSourceNow(source: ScrapeSourceView, event?: Event): void {
    event?.stopPropagation();
    if (!this.isAdmin || !source?.id) {
      return;
    }

    this.scrapingSources[source.id] = true;
    this.http.requeueScrapeSource(source.id).subscribe({
      next: res => {
        this.notification.showSuccess(res?.message ?? 'Scrape source queued successfully');
      },
      error: err => {
        this.notification.showError('Could not queue scrape source: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
      }
    }).add(() => {
      delete this.scrapingSources[source.id];
    });
  }

  private buildViewSource(source: ScrapeSourceInfo): ScrapeSourceView {
    const { head, tail } = this.splitUrlForDisplay(source.url);
    return {
      ...source,
      urlHead: head,
      urlTail: tail,
    };
  }

  private applySort(
    sources: ScrapeSourceView[],
    field: string | null,
    order: number | null,
  ): ScrapeSourceView[] {
    if (!field || !order || sources.length < 2) {
      return sources;
    }

    const direction = order > 0 ? 1 : -1;
    return [...sources].sort((left, right) => this.compareSources(left, right, field, direction));
  }

  private compareSources(
    left: ScrapeSourceView,
    right: ScrapeSourceView,
    field: string,
    direction: number,
  ): number {
    const leftValue = this.getSortableValue(left, field);
    const rightValue = this.getSortableValue(right, field);

    if (typeof leftValue === 'number' && typeof rightValue === 'number') {
      if (leftValue === rightValue) {
        return left.url.localeCompare(right.url);
      }
      return (leftValue - rightValue) * direction;
    }

    const comparison = String(leftValue).localeCompare(String(rightValue), undefined, {
      numeric: true,
      sensitivity: 'base',
    });
    if (comparison === 0) {
      return left.url.localeCompare(right.url) * direction;
    }
    return comparison * direction;
  }

  private getSortableValue(source: ScrapeSourceView, field: string): number | string {
    switch (field) {
      case 'proxy_count':
        return source.proxy_count ?? 0;
      case 'health':
        return this.calculateHealthScore(source);
      case 'url':
      default:
        return source.url ?? '';
    }
  }

  private calculateHealthScore(source: ScrapeSourceView): number {
    const total = source.proxy_count ?? 0;
    if (total <= 0) {
      return -1;
    }

    const alive = source.alive_count ?? 0;
    return alive / total;
  }

  private resolveSortField(field: string | string[] | undefined | null): string | null {
    if (typeof field === 'string' && field.trim().length > 0) {
      return field;
    }

    if (Array.isArray(field)) {
      const first = field.find(value => value.trim().length > 0);
      return first ?? null;
    }

    return null;
  }

  private splitUrlForDisplay(url: string | null | undefined): { head: string; tail: string } {
    const safeUrl = (url ?? '').trim();
    if (!safeUrl) {
      return { head: '', tail: '' };
    }

    const minLengthForSplit = 36;
    if (safeUrl.length <= minLengthForSplit) {
      return { head: safeUrl, tail: '' };
    }

    const queryIndex = safeUrl.indexOf('?');
    if (queryIndex >= 0 && queryIndex < safeUrl.length - 1) {
      const base = safeUrl.slice(0, queryIndex);
      const query = safeUrl.slice(queryIndex + 1);
      const queryTailLength = 24;

      if (query.length > queryTailLength) {
        return {
          head: `${base}?${query.slice(0, query.length - queryTailLength)}`,
          tail: query.slice(-queryTailLength)
        };
      }
    }

    const trimmed = safeUrl.endsWith('/') ? safeUrl.slice(0, -1) : safeUrl;
    const schemeIndex = trimmed.indexOf('://');
    const hostStart = schemeIndex >= 0 ? schemeIndex + 3 : 0;
    const lastSlash = trimmed.lastIndexOf('/');

    if (lastSlash > hostStart && lastSlash < trimmed.length - 1) {
      const head = trimmed.slice(0, lastSlash);
      const tail = trimmed.slice(lastSlash) + (safeUrl.endsWith('/') ? '/' : '');
      return { head, tail };
    }

    const fallbackTailLength = 12;
    if (safeUrl.length <= fallbackTailLength) {
      return { head: safeUrl, tail: '' };
    }

    return {
      head: safeUrl.slice(0, safeUrl.length - fallbackTailLength),
      tail: safeUrl.slice(-fallbackTailLength)
    };
  }

  private syncSelectionWithData(): void {
    const selectedIds = new Set(this.selection.selected.map(source => source.id));
    this.selection.clear();

    this.scrapeSources.forEach(source => {
      if (selectedIds.has(source.id)) {
        this.selection.select(source);
      }
    });
    this.selectedScrapeSources = [...this.selection.selected];
  }

  private syncColumnsFromSettings(settings: UserSettings | undefined): void {
    const normalized = normalizeScrapeSourceListColumns(settings?.scrape_source_list_columns ?? DEFAULT_SCRAPE_SOURCE_LIST_COLUMNS);
    this.displayedColumns = normalized;
  }

  private resolveColumnPickerColumns(): readonly ScrapeSourceListColumnDefinition[] {
    if (this.isAdmin) {
      return this.scrapeSourceColumnDefinitions;
    }
    return this.scrapeSourceColumnDefinitions.filter(column => column.id !== 'scrape_now');
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
