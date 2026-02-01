import {Component, EventEmitter, OnInit, Output} from '@angular/core';
import {DatePipe} from '@angular/common';
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

@Component({
  selector: 'app-scrape-source-list',
  imports: [
    DatePipe,
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

  scrapeSources: ScrapeSourceInfo[] = [];
  selection = new SelectionModel<ScrapeSourceInfo>(true, []);
  selectedScrapeSources: ScrapeSourceInfo[] = [];
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
    private router: Router
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
        NotificationService.showWarn('Could not load robots.txt setting: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
        this.respectRobotsEnabled = false;
      }
    });
  }

  getAndSetScrapeSourcesList() {
    this.loading = true;
    this.http.getScrapingSourcePage(this.page + 1).subscribe({
      next: res => {
        const sources = Array.isArray(res) ? res : [];
        this.scrapeSources = sources;
        this.syncSelectionWithData();
        this.loading = false;
        this.hasLoaded = true;
        const shouldShowEmptyState = this.totalItems === 0 && sources.length === 0;
        this.showAddScrapeSourceMessage.emit(shouldShowEmptyState);
      },
      error: err => {
        NotificationService.showError("Could not get scraping sources" + err.error.message);
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
        NotificationService.showError("Could not get scrape sources count " + err.error.message);
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
            NotificationService.showSuccess(res);
            this.totalItems -= selected.length;
            this.selection.clear();
            this.selectedScrapeSources = [];
            this.getAndSetScrapeSourcesList();
          },
          error: err => NotificationService.showError("Could not delete scraping source " + err.error.message)
        });
      }
    });
  }

  // Helper method to get selection count
  getSelectionCount(): number {
    return this.selection.selected.length;
  }

  toggleSelection(source: ScrapeSourceInfo): void {
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

  onViewSource(event: Event | { originalEvent?: Event }, source: ScrapeSourceInfo): void {
    if ((event as { originalEvent?: Event }).originalEvent) {
      (event as { originalEvent?: Event }).originalEvent?.stopPropagation?.();
    } else {
      (event as Event)?.stopPropagation?.();
    }
    this.router.navigate(['/scraper', source.id]).catch(() => {});
  }

  checkRobots(source: ScrapeSourceInfo, event?: Event): void {
    event?.stopPropagation();
    if (!source?.url) {
      return;
    }

    this.checkingRobots[source.id] = true;
    this.http.checkScrapeSource(source.url).subscribe({
      next: res => {
        const { allowed, robots_found, error } = res ?? { allowed: true, robots_found: false };

        if (allowed && robots_found) {
          NotificationService.showSuccess('robots.txt allows scraping this URL');
        } else if (!allowed && robots_found) {
          NotificationService.showWarn('robots.txt disallows scraping this URL');
        } else if (allowed && !robots_found) {
          NotificationService.showInfo('No robots.txt found; scraping is allowed by default');
        }

        if (error) {
          NotificationService.showWarn('Robots check completed with warnings: ' + error);
        }
      },
      error: err => {
        NotificationService.showError('Could not check robots.txt: ' + (err?.error?.error ?? err?.message ?? 'Unknown error'));
      }
    }).add(() => {
      delete this.checkingRobots[source.id];
    });
  }

  isCheckingRobots(sourceId: number): boolean {
    return this.checkingRobots[sourceId];
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
}
