import {CommonModule} from '@angular/common';
import {Component, Input} from '@angular/core';

type HealthTone = 'healthy' | 'mixed' | 'unhealthy' | 'empty';

@Component({
  selector: 'app-health-bar-cell',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './health-bar-cell.component.html',
  styleUrl: './health-bar-cell.component.scss',
})
export class HealthBarCellComponent {
  @Input() aliveCount: number | null | undefined = 0;
  @Input() deadCount: number | null | undefined = null;
  @Input() unknownCount: number | null | undefined = null;
  @Input() showUnknown = true;
  @Input() total: number | null | undefined = null;
  @Input() hasData: boolean | null = null;
  @Input() valueSuffix = '';
  @Input() valueDecimals = 0;

  hovered: { x: number; y: number } | null = null;

  get ratioLabel(): string {
    if (!this.dataAvailable) {
      return 'No data';
    }
    return `${Math.round(this.aliveRatio * 100)}% alive`;
  }

  get statusLabel(): string {
    if (this.tone === 'healthy') {
      return 'Healthy';
    }
    if (this.tone === 'mixed') {
      return 'Mixed';
    }
    if (this.tone === 'unhealthy') {
      return 'Unhealthy';
    }
    return 'No data';
  }

  get statusPillClass(): string {
    if (this.tone === 'mixed') {
      return 'status-pill--mixed';
    }
    if (this.tone === 'unhealthy') {
      return 'status-pill--dead';
    }
    if (this.tone === 'empty') {
      return 'status-pill--unknown';
    }
    return '';
  }

  get dotClass(): Record<string, boolean> {
    return {
      alive: this.tone === 'healthy',
      mixed: this.tone === 'mixed',
      dead: this.tone === 'unhealthy',
      unknown: this.tone === 'empty',
    };
  }

  get alivePercent(): number {
    if (!this.dataAvailable || this.resolvedTotal <= 0) {
      return 0;
    }
    return (this.resolvedAlive / this.resolvedTotal) * 100;
  }

  get deadPercent(): number {
    if (!this.dataAvailable || this.resolvedTotal <= 0) {
      return 0;
    }
    return (this.resolvedDead / this.resolvedTotal) * 100;
  }

  get unknownPercent(): number {
    if (!this.showUnknown) {
      return 0;
    }
    if (!this.dataAvailable) {
      return 100;
    }
    if (this.resolvedTotal <= 0) {
      return 0;
    }
    return (this.resolvedUnknown / this.resolvedTotal) * 100;
  }

  get aliveDisplay(): string {
    return this.formatValue(this.resolvedAlive);
  }

  get deadDisplay(): string {
    return this.formatValue(this.resolvedDead);
  }

  get unknownDisplay(): string {
    return this.formatValue(this.resolvedUnknown);
  }

  showHealthPopup(event: MouseEvent): void {
    const target = event.currentTarget as HTMLElement | null;
    if (!target) {
      return;
    }
    const rect = target.getBoundingClientRect();
    const spacing = 10;
    this.hovered = {
      x: rect.left + rect.width / 2,
      y: rect.bottom + spacing,
    };
  }

  hideHealthPopup(): void {
    this.hovered = null;
  }

  private get resolvedAlive(): number {
    return this.normalize(this.aliveCount);
  }

  private get resolvedDead(): number {
    if (this.deadCount === null || this.deadCount === undefined) {
      if (this.providedTotal > 0) {
        return Math.max(0, this.providedTotal - this.resolvedAlive - this.resolvedUnknown);
      }
      return 0;
    }
    return this.normalize(this.deadCount);
  }

  private get resolvedUnknown(): number {
    if (!this.showUnknown) {
      return 0;
    }
    if (this.unknownCount === null || this.unknownCount === undefined) {
      if (this.providedTotal > 0 && (this.deadCount !== null && this.deadCount !== undefined)) {
        return Math.max(0, this.providedTotal - this.resolvedAlive - this.normalize(this.deadCount));
      }
      return 0;
    }
    return this.normalize(this.unknownCount);
  }

  private get providedTotal(): number {
    return this.normalize(this.total);
  }

  private get resolvedTotal(): number {
    if (this.providedTotal > 0) {
      return this.providedTotal;
    }
    return this.resolvedAlive + this.resolvedDead + this.resolvedUnknown;
  }

  private get dataAvailable(): boolean {
    if (this.hasData !== null) {
      return this.hasData;
    }
    return this.resolvedTotal > 0;
  }

  private get aliveRatio(): number {
    if (!this.dataAvailable || this.resolvedTotal <= 0) {
      return 0;
    }
    return this.resolvedAlive / this.resolvedTotal;
  }

  private get tone(): HealthTone {
    if (!this.dataAvailable) {
      return 'empty';
    }
    const ratio = this.aliveRatio;
    if (ratio >= 0.7) {
      return 'healthy';
    }
    if (ratio >= 0.4) {
      return 'mixed';
    }
    return 'unhealthy';
  }

  private normalize(value: number | null | undefined): number {
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      return 0;
    }
    return Math.max(0, value);
  }

  private formatValue(value: number): string {
    if (this.valueDecimals <= 0) {
      return `${Math.round(value)}${this.valueSuffix}`;
    }
    const rounded = Math.round(value * Math.pow(10, this.valueDecimals)) / Math.pow(10, this.valueDecimals);
    return `${rounded.toFixed(this.valueDecimals)}${this.valueSuffix}`;
  }
}
