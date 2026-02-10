import {Component, EventEmitter, OnInit, Output} from '@angular/core';
import {CommonModule} from '@angular/common';
import {FormsModule} from '@angular/forms';
import {SelectionModel} from '@angular/cdk/collections';
import {Router} from '@angular/router';
import {HttpService} from '../../services/http.service';
import {ScrapeSourceInfo} from '../../models/ScrapeSourceInfo';
import {AddScrapeSourceComponent} from '../add-scrape-source/add-scrape-source.component';

// PrimeNG imports
import {TableLazyLoadEvent, TableModule} from 'primeng/table';
import {ButtonModule} from 'primeng/button';
import {CheckboxModule} from 'primeng/checkbox';
import {ConfirmDialogModule} from 'primeng/confirmdialog';
import {SkeletonModule} from 'primeng/skeleton';
import {ConfirmationService} from 'primeng/api';
import {NotificationService} from '../../services/notification-service.service';

type HealthTone = 'healthy' | 'mixed' | 'unhealthy' | 'empty';

interface ScrapeSourceHealthView {
  tone: HealthTone;
  label: string;
  ratioLabel: string;
  dotClass: Record<string, boolean>;
  aliveCount: number;
  deadCount: number;
  unknownCount: number;
  alivePercent: number;
  deadPercent: number;
  unknownPercent: number;
}

interface ScrapeSourceView extends ScrapeSourceInfo {
  urlHead: string;
  urlTail: string;
  health: ScrapeSourceHealthView;
}

@Component({
  selector: 'app-scrape-source-list',
  imports: [
    CommonModule,
    FormsModule,
    TableModule,
    ButtonModule,
    CheckboxModule,
    ConfirmDialogModule,
    SkeletonModule,
    AddScrapeSourceComponent
  ],
  providers: [ConfirmationService],
  templateUrl: './scrape-source-list.component.html',
  styleUrl: './scrape-source-list.component.scss',
  standalone: true
})
export class ScrapeSourceListComponent implements OnInit {
  @Output() showAddScrapeSourceMessage = new EventEmitter<boolean>();

  scrapeSources: ScrapeSourceView[] = [];
  hoveredHealth: { source: ScrapeSourceView; x: number; y: number } | null = null;
  selection = new SelectionModel<ScrapeSourceView>(true, []);
  selectedScrapeSources: ScrapeSourceView[] = [];
  page = 0; // PrimeNG uses 0-based pagination
  pageSize = 20;
  totalItems = 0;
  hasLoaded = false;
  loading = false;
  checkingRobots: Record<number, boolean> = {};
  respectRobotsEnabled = false;
  readonly skeletonRows = Array.from({ length: 6 });

  constructor(
    private http: HttpService,
    private confirmationService: ConfirmationService,
    private router: Router,
    private notification: NotificationService
  ) { }

  ngOnInit(): void {
    this.loadRespectRobotsSetting();
    this.getAndSetScrapeSourceCount();
    this.getAndSetScrapeSourcesList();
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
        this.scrapeSources = sources.map(source => this.buildViewSource(source));
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

    const shouldFetch = newPage !== this.page || newPageSize !== this.pageSize;

    this.page = newPage;
    this.pageSize = newPageSize;

    if (shouldFetch) {
      this.getAndSetScrapeSourcesList();
    }
  }

  deleteSelectedSources(): void {
    const selected = [...this.selection.selected];
    if (selected.length === 0) {
      return;
    }

    this.confirmationService.confirm({
      message: `Are you sure you want to delete ${selected.length} selected scrape source(s)?`,
      header: 'Confirm Deletion',
      icon: 'pi pi-exclamation-triangle',
      acceptButtonStyleClass: 'p-button-danger',
      accept: () => {
        const selectedIds = selected.map(source => source.id);

        this.http.deleteScrapingSource(selectedIds).subscribe({
          next: res => {
            this.notification.showSuccess(res);
            this.totalItems -= selected.length;
            this.selection.clear();
            this.selectedScrapeSources = [];
            this.getAndSetScrapeSourcesList();
          },
          error: err => this.notification.showError("Could not delete scraping source " + err.error.message)
        });
      }
    });
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

  onScrapeSourcesAdded(): void {
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

  showHealthPopup(event: MouseEvent, source: ScrapeSourceView): void {
    const target = event.currentTarget as HTMLElement | null;
    if (!target) {
      return;
    }
    const bar = target.querySelector<HTMLElement>('.health-bar');
    const rect = (bar ?? target).getBoundingClientRect();
    const spacing = 8;
    this.hoveredHealth = {
      source,
      x: rect.left + rect.width / 2,
      y: rect.top - spacing,
    };
  }

  hideHealthPopup(): void {
    this.hoveredHealth = null;
  }

  private buildViewSource(source: ScrapeSourceInfo): ScrapeSourceView {
    const { head, tail } = this.splitUrlForDisplay(source.url);
    return {
      ...source,
      urlHead: head,
      urlTail: tail,
      health: this.buildHealthView(source)
    };
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

  private buildHealthView(source: ScrapeSourceInfo): ScrapeSourceHealthView {
    const total = Math.max(0, source.proxy_count ?? 0);
    const alive = Math.max(0, source.alive_count ?? 0);
    const dead = Math.max(0, source.dead_count ?? 0);
    const unknownFallback = Math.max(0, total - alive - dead);
    const unknown = Math.max(0, source.unknown_count ?? unknownFallback);

    let tone: HealthTone = 'empty';
    if (total > 0) {
      const ratio = alive / total;
      if (ratio >= 0.7) {
        tone = 'healthy';
      } else if (ratio >= 0.4) {
        tone = 'mixed';
      } else {
        tone = 'unhealthy';
      }
    }

    const label = tone === 'healthy'
      ? 'Healthy'
      : tone === 'mixed'
        ? 'Mixed'
        : tone === 'unhealthy'
          ? 'Unhealthy'
          : 'No data';

    const ratioLabel = total > 0 ? `${Math.round((alive / total) * 100)}% alive` : 'No data';

    const dotClass = {
      alive: tone === 'healthy',
      mixed: tone === 'mixed',
      dead: tone === 'unhealthy',
      unknown: tone === 'empty',
    };

    const alivePercent = total > 0 ? (alive / total) * 100 : 0;
    const deadPercent = total > 0 ? (dead / total) * 100 : 0;
    const unknownPercent = total > 0 ? (unknown / total) * 100 : 0;

    return {
      tone,
      label,
      ratioLabel,
      dotClass,
      aliveCount: alive,
      deadCount: dead,
      unknownCount: unknown,
      alivePercent,
      deadPercent,
      unknownPercent,
    };
  }
}
