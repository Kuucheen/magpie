import {ChangeDetectionStrategy, Component, EventEmitter, Input, OnChanges, Output, SimpleChanges} from '@angular/core';
import {NgClass} from '@angular/common';
import {SelectionModel} from '@angular/cdk/collections';
import {TableLazyLoadEvent, TableModule} from 'primeng/table';
import {SkeletonModule} from 'primeng/skeleton';
import {Tooltip} from 'primeng/tooltip';
import {ProxyInfo} from '../../models/ProxyInfo';
import {ProxyReputation} from '../../models/ProxyReputation';

interface ProxyRowMeta {
  hasReputation: boolean;
  reputationBadgeClass: string;
  reputationLabel: string;
  reputationScore: string;
  latestCheckLabel: string;
}

type ProxyRow = ProxyInfo & { __meta?: ProxyRowMeta };

@Component({
  selector: 'app-proxy-table',
  standalone: true,
  imports: [
    NgClass,
    TableModule,
    SkeletonModule,
    Tooltip,
  ],
  templateUrl: './proxy-table.component.html',
  styleUrls: ['./proxy-table.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProxyTableComponent implements OnChanges {
  private _proxies: ProxyRow[] = [];
  private readonly dateFormatter = new Intl.DateTimeFormat(undefined, {
    dateStyle: 'short',
    timeStyle: 'short',
  });

  @Input()
  set proxies(value: ProxyInfo[]) {
    this._proxies = (value ?? []) as ProxyRow[];
    this.decorateProxies();
  }
  get proxies(): ProxyRow[] {
    return this._proxies;
  }
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
  @Input() virtualScroll = false;
  @Input() virtualScrollItemSize = 46;
  @Input() scrollHeight: string | null = null;

  @Output() lazyLoad = new EventEmitter<TableLazyLoadEvent>();
  @Output() selectionChange = new EventEmitter<ProxyInfo[]>();
  @Output() toggleSelection = new EventEmitter<ProxyInfo>();
  @Output() masterToggle = new EventEmitter<void>();
  @Output() sort = new EventEmitter<{ field: string; order: number }>();
  @Output() viewProxy = new EventEmitter<ProxyInfo>();

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['emptyReputationLabel'] || changes['missingReputationScoreLabel']) {
      this.decorateProxies();
    }
  }

  get columnCount(): number {
    return this.selectionEnabled ? 10 : 9;
  }

  get scoreFallback(): string {
    return this.missingReputationScoreLabel ?? this.emptyReputationLabel;
  }

  trackByProxy(_index: number, proxy: ProxyRow): number {
    return proxy.id;
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

  private decorateProxies(): void {
    if (!this._proxies.length) {
      return;
    }
    for (const proxy of this._proxies) {
      this.buildMeta(proxy);
    }
  }

  private buildMeta(proxy: ProxyRow): ProxyRowMeta {
    const reputation = this.getPrimaryReputation(proxy);
    if (!reputation) {
      proxy.__meta = {
        hasReputation: false,
        reputationBadgeClass: 'reputation-badge reputation-badge--unknown',
        reputationLabel: 'Unknown',
        reputationScore: this.scoreFallback,
        latestCheckLabel: this.formatLatestCheck(proxy.latest_check),
      };
      return proxy.__meta;
    }

    const rawLabel = `${reputation.label ?? ''}`.trim();
    const label = rawLabel.length > 0 ? rawLabel : 'Unknown';
    const normalized = label.toLowerCase();
    let badgeClass = 'reputation-badge reputation-badge--unknown';
    if (normalized === 'good') {
      badgeClass = 'reputation-badge reputation-badge--good';
    } else if (normalized === 'neutral') {
      badgeClass = 'reputation-badge reputation-badge--neutral';
    } else if (normalized === 'poor') {
      badgeClass = 'reputation-badge reputation-badge--poor';
    }

    const score = reputation.score === null || reputation.score === undefined
      ? this.scoreFallback
      : Math.round(reputation.score).toString();

    proxy.__meta = {
      hasReputation: true,
      reputationBadgeClass: badgeClass,
      reputationLabel: label,
      reputationScore: score,
      latestCheckLabel: this.formatLatestCheck(proxy.latest_check),
    };
    return proxy.__meta;
  }

  private formatLatestCheck(value: unknown): string {
    if (!value) {
      return '—';
    }
    const date = value instanceof Date ? value : new Date(value as string);
    if (!Number.isFinite(date.getTime())) {
      return '—';
    }
    return this.dateFormatter.format(date);
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
