import {Component, Input} from '@angular/core';
import {Card} from 'primeng/card';
import {PrimeTemplate} from 'primeng/api';
import {DecimalPipe, NgStyle} from '@angular/common';
import {UIChart} from 'primeng/chart';

interface ReputationBreakdown {
  good: number;
  neutral: number;
  poor: number;
  unknown: number;
}

@Component({
  selector: 'app-proxy-reputation-card',
  standalone: true,
  imports: [Card, PrimeTemplate, UIChart, NgStyle, DecimalPipe],
  templateUrl: './proxy-reputation-card.component.html',
  styleUrl: './proxy-reputation-card.component.scss'
})
export class ProxyReputationCardComponent {
  @Input({ required: true }) breakdown!: ReputationBreakdown;
  @Input({ required: true }) chartData!: any;
  @Input({ required: true }) chartOptions!: any;

  readonly cardStyleClass = 'chart-card bg-neutral-900 border border-neutral-800 reputation-card';
  readonly labels: Array<{ key: keyof ReputationBreakdown; title: string }> = [
    { key: 'good', title: 'Good' },
    { key: 'neutral', title: 'Neutral' },
    { key: 'poor', title: 'Poor' },
    { key: 'unknown', title: 'Unknown' }
  ];

  get total(): number {
    if (!this.breakdown) {
      return 0;
    }
    return (
      (this.breakdown.good ?? 0) +
      (this.breakdown.neutral ?? 0) +
      (this.breakdown.poor ?? 0) +
      (this.breakdown.unknown ?? 0)
    );
  }

  get entries(): Array<{
    key: keyof ReputationBreakdown;
    title: string;
    value: number;
    percentage: number;
    color: string;
  }> {
    const total = this.total;
    const datasets = Array.isArray(this.chartData?.datasets) ? this.chartData.datasets : [];
    const primaryColors = Array.isArray(datasets?.[0]?.backgroundColor) ? datasets[0].backgroundColor : [];
    const accentColors = Array.isArray(datasets?.[1]?.backgroundColor) ? datasets[1].backgroundColor : [];

    return this.labels.map((entry, index) => {
      const raw = this.breakdown?.[entry.key] ?? 0;
      const color = accentColors[index] ?? primaryColors[index] ?? 'rgba(148,163,184,0.6)';
      return {
        key: entry.key,
        title: entry.title,
        value: raw,
        percentage: total > 0 ? Math.round((raw / total) * 1000) / 10 : 0,
        color
      };
    });
  }

  trackByKey(_: number, item: { key: keyof ReputationBreakdown }): string {
    return item.key;
  }

  proxyBadgeClass(label: string | null | undefined): string {
    const normalized = (label ?? '').toLowerCase();
    if (normalized === 'good') {
      return 'badge badge--good';
    }
    if (normalized === 'neutral') {
      return 'badge badge--neutral';
    }
    if (normalized === 'poor') {
      return 'badge badge--poor';
    }
    return 'badge';
  }
}
