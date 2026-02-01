import {Component, EventEmitter, Input, Output} from '@angular/core';
import {DatePipe, NgClass} from '@angular/common';
import {FormsModule} from '@angular/forms';
import {SelectionModel} from '@angular/cdk/collections';
import {TableLazyLoadEvent, TableModule} from 'primeng/table';
import {CheckboxModule} from 'primeng/checkbox';
import {SkeletonModule} from 'primeng/skeleton';
import {ButtonModule} from 'primeng/button';
import {Tooltip} from 'primeng/tooltip';
import {ProxyInfo} from '../../models/ProxyInfo';
import {ProxyReputation} from '../../models/ProxyReputation';

@Component({
  selector: 'app-proxy-table',
  standalone: true,
  imports: [
    DatePipe,
    NgClass,
    FormsModule,
    TableModule,
    CheckboxModule,
    SkeletonModule,
    ButtonModule,
    Tooltip,
  ],
  templateUrl: './proxy-table.component.html',
  styleUrls: ['./proxy-table.component.scss'],
})
export class ProxyTableComponent {
  @Input() proxies: ProxyInfo[] = [];
  @Input() loading = false;
  @Input() hasLoaded = false;
  @Input() page = 1;
  @Input() pageSize = 20;
  @Input() totalRecords = 0;
  @Input() rowsPerPageOptions: number[] = [20, 50];
  @Input() skeletonRows: unknown[] = [];

  @Input() selectionEnabled = false;
  @Input() selection: ProxyInfo[] = [];
  @Input() selectionModel?: SelectionModel<ProxyInfo>;
  @Input() isAllSelected = false;

  @Input() sortable = false;
  @Input() sortField: string | null = null;
  @Input() sortOrder: number | null = null;

  @Input() emptyReputationLabel = '—';
  @Input() missingReputationScoreLabel: string | null = null;

  @Output() lazyLoad = new EventEmitter<TableLazyLoadEvent>();
  @Output() selectionChange = new EventEmitter<ProxyInfo[]>();
  @Output() toggleSelection = new EventEmitter<ProxyInfo>();
  @Output() masterToggle = new EventEmitter<void>();
  @Output() sort = new EventEmitter<{ field: string; order: number }>();
  @Output() viewProxy = new EventEmitter<ProxyInfo>();

  get columnCount(): number {
    return this.selectionEnabled ? 10 : 9;
  }

  get scoreFallback(): string {
    return this.missingReputationScoreLabel ?? this.emptyReputationLabel;
  }

  onSelectionChange(value: ProxyInfo[]): void {
    if (!this.selectionEnabled) {
      return;
    }
    this.selectionChange.emit(value);
  }

  onMasterToggle(): void {
    if (!this.selectionEnabled) {
      return;
    }
    this.masterToggle.emit();
  }

  onToggleSelection(proxy: ProxyInfo): void {
    if (!this.selectionEnabled) {
      return;
    }
    this.toggleSelection.emit(proxy);
  }

  onSort(event: { field: string; order: number }): void {
    if (!this.sortable) {
      return;
    }
    this.sort.emit(event);
  }

  onViewProxy(event: Event | { originalEvent?: Event }, proxy: ProxyInfo): void {
    if ((event as { originalEvent?: Event }).originalEvent) {
      (event as { originalEvent?: Event }).originalEvent?.stopPropagation?.();
    } else {
      (event as Event)?.stopPropagation?.();
    }
    this.viewProxy.emit(proxy);
  }

  hasReputation(proxy: ProxyInfo): boolean {
    return this.getPrimaryReputation(proxy) !== null;
  }

  reputationBadgeClass(proxy: ProxyInfo): string {
    const label = this.getPrimaryReputation(proxy)?.label?.toLowerCase() ?? '';
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

  reputationLabel(proxy: ProxyInfo): string {
    const label = this.getPrimaryReputation(proxy)?.label?.trim();
    if (label && label.length > 0) {
      return label;
    }
    return 'Unknown';
  }

  reputationScore(proxy: ProxyInfo): string {
    const score = this.getPrimaryReputation(proxy)?.score;
    if (score === null || score === undefined) {
      return this.scoreFallback;
    }
    return Math.round(score).toString();
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
}
