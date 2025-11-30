import {Component, Input, OnChanges, SimpleChanges} from '@angular/core';
import {NgStyle} from '@angular/common';
import {Card} from 'primeng/card';
import {PrimeTemplate} from 'primeng/api';
import {UIChart} from 'primeng/chart';
import Chart from 'chart.js/auto';
import {ChartData, ChartOptions, TooltipItem} from 'chart.js';
import {ChoroplethController, GeoFeature, ColorScale, ProjectionScale} from 'chartjs-chart-geo';
import {feature} from 'topojson-client';
import worldMap from 'world-atlas/countries-110m.json';
import {Feature, FeatureCollection} from 'geojson';

interface CountryBreakdown {
  name: string;
  percentage: string | number;
  value?: number;
  color?: string;
}

type CountryFeature = Feature & { properties: { name: string } };

Chart.register(ChoroplethController, GeoFeature, ColorScale, ProjectionScale);

const WORLD_TOPO = worldMap as unknown as { objects: { countries: any } };
const WORLD_FEATURE_COLLECTION = feature(
  WORLD_TOPO as any,
  WORLD_TOPO.objects.countries
) as unknown as FeatureCollection;
const WORLD_FEATURES_ALL = WORLD_FEATURE_COLLECTION.features as CountryFeature[];
const WORLD_FEATURES = WORLD_FEATURES_ALL.filter(
  (feat) => (feat.properties?.name ?? '').toLowerCase() !== 'antarctica'
) as CountryFeature[];
const FEATURE_BY_NAME = new Map<string, CountryFeature>();

WORLD_FEATURES.forEach((feat) => {
  const key = (feat.properties?.name ?? '').toString().toLowerCase();
  if (key) {
    FEATURE_BY_NAME.set(key, feat);
  }
});

const COUNTRY_ALIASES: Record<string, string> = {
  usa: 'United States of America',
  'united states': 'United States of America',
  'united states of america': 'United States of America',
  uk: 'United Kingdom',
  'south korea': 'South Korea',
  'north korea': 'North Korea',
  russia: 'Russia',
  'czech republic': 'Czechia',
  laos: 'Laos',
  'lao people\'s democratic republic': 'Laos',
  vietnam: 'Vietnam',
  venezuela: 'Venezuela',
  bolivia: 'Bolivia',
  tanzania: 'Tanzania',
  syria: 'Syria',
  iran: 'Iran',
  moldova: 'Moldova',
  palestine: 'Palestine',
  'palestinian territories': 'Palestine',
  macedonia: 'Macedonia',
  'north macedonia': 'Macedonia',
  'bosnia and herzegovina': 'Bosnia and Herz.',
  'democratic republic of the congo': 'Dem. Rep. Congo',
  'republic of the congo': 'Congo',
  congo: 'Congo',
  ivorycoast: "Côte d'Ivoire",
  "cote d'ivoire": "Côte d'Ivoire",
  'ivory coast': "Côte d'Ivoire",
  swaziland: 'eSwatini',
  eswatini: 'eSwatini'
};

Object.entries(COUNTRY_ALIASES).forEach(([alias, canonical]) => {
  const target = FEATURE_BY_NAME.get(canonical.toLowerCase());
  if (target) {
    FEATURE_BY_NAME.set(alias, target);
  }
});

@Component({
  selector: 'app-proxies-per-country-card',
  standalone: true,
  imports: [Card, PrimeTemplate, UIChart, NgStyle],
  templateUrl: './proxies-per-country-card.component.html',
  styleUrl: './proxies-per-country-card.component.scss'
})
export class ProxiesPerCountryCardComponent implements OnChanges {
  @Input() title = 'Proxies per country';
  @Input() countries: CountryBreakdown[] = [];
  @Input() styleClass = 'chart-card bg-neutral-900 border border-neutral-800 mb-4';

  viewMode: 'map' | 'countries' = 'map';
  mapData: ChartData<'choropleth'> = { labels: [], datasets: [] };
  mapOptions: ChartOptions<'choropleth'> = {};
  maxCountryValue = 1;
  readonly mapChartType: any = 'choropleth';

  private readonly regionNames = typeof Intl.DisplayNames !== 'undefined'
    ? new Intl.DisplayNames(['en'], { type: 'region' })
    : null;

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['countries']) {
      this.buildMap();
    }
  }

  previewColor(country: CountryBreakdown): string {
    const normalized = Math.min(1, Math.max(0, this.resolveValue(country) / this.maxCountryValue));
    return this.interpolateColor(normalized);
  }

  setViewMode(mode: 'map' | 'countries'): void {
    this.viewMode = mode;
  }

  private buildMap(): void {
    const values = new Map<string, number>();
    let maxValue = 0;

    for (const entry of this.countries ?? []) {
      const featureMatch = this.resolveFeature(entry.name);
      if (!featureMatch) {
        continue;
      }

      const value = this.resolveValue(entry);
      if (value <= 0) {
        continue;
      }

      const key = (featureMatch.properties?.name ?? '').toLowerCase();
      const nextValue = (values.get(key) ?? 0) + value;
      values.set(key, nextValue);
      maxValue = Math.max(maxValue, nextValue);
    }

    this.maxCountryValue = Math.max(maxValue, 1);

    const dataset = WORLD_FEATURES.map((feature) => {
      const key = (feature.properties?.name ?? '').toLowerCase();
      return {
        feature,
        value: values.get(key) ?? 0
      };
    });

    this.mapData = {
      labels: WORLD_FEATURES.map((feat) => feat.properties?.name ?? ''),
      datasets: [
        {
          label: 'Proxies',
          outline: WORLD_FEATURES as any[],
          data: dataset,
          borderColor: 'rgba(255, 255, 255, 0.18)',
          borderWidth: 0.8,
          hoverBorderColor: 'rgba(255, 255, 255, 0.85)',
          hoverBorderWidth: 1.6,
          hoverBackgroundColor: (context: any) => {
            const raw = context.raw as { value?: number } | undefined;
            const value = typeof raw?.value === 'number' ? raw.value : 0;
            const normalized = this.maxCountryValue > 0 ? value / this.maxCountryValue : 0;
            return this.interpolateColor(Math.min(1, normalized + 0.25));
          }
        }
      ]
    };

    this.mapOptions = this.createMapOptions(this.maxCountryValue);
  }

  private resolveFeature(name: string | undefined | null): CountryFeature | undefined {
    const normalized = (name ?? '').trim();
    if (!normalized || normalized.toLowerCase() === 'unknown' || normalized.toLowerCase() === 'others') {
      return undefined;
    }

    const lower = normalized.toLowerCase();
    const exact = FEATURE_BY_NAME.get(lower);
    if (exact) {
      return exact;
    }

    const alias = COUNTRY_ALIASES[lower];
    if (alias) {
      const mapped = FEATURE_BY_NAME.get(alias.toLowerCase());
      if (mapped) {
        return mapped;
      }
    }

    if (this.regionNames && /^[a-z]{2,3}$/i.test(normalized)) {
      const display = this.regionNames.of(normalized.toUpperCase());
      if (display) {
        const displayKey = display.toLowerCase();
        const displayMatch = FEATURE_BY_NAME.get(displayKey)
          ?? FEATURE_BY_NAME.get((COUNTRY_ALIASES[displayKey] ?? '').toLowerCase());
        if (displayMatch) {
          return displayMatch;
        }
      }
    }

    return WORLD_FEATURES.find((feat) => {
      const key = (feat.properties?.name ?? '').toString().toLowerCase();
      return key === lower || key.startsWith(lower) || lower.startsWith(key);
    });
  }

  private resolveValue(country: CountryBreakdown): number {
    if (typeof country.value === 'number' && !Number.isNaN(country.value)) {
      return country.value;
    }

    if (typeof country.percentage === 'number' && !Number.isNaN(country.percentage)) {
      return country.percentage;
    }

    const parsed = Number.parseFloat(country.percentage?.toString() ?? '0');
    return Number.isFinite(parsed) ? parsed : 0;
  }

  private createMapOptions(maxValue: number): ChartOptions<'choropleth'> {
    const max = Math.max(maxValue, 1);
    return {
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (context: TooltipItem<'choropleth'>) => {
              const raw = context.raw as any;
              const feature = raw?.feature as CountryFeature | undefined;
              const label = feature?.properties?.name ?? context.label ?? 'Unknown';
              const value = typeof raw?.value === 'number' ? raw.value : 0;
              return `${label}: ${value}`;
            }
          },
          backgroundColor: '#0b1220',
          titleColor: '#e5e7eb',
          bodyColor: '#cbd5e1',
          borderColor: '#1f2937',
          borderWidth: 1,
          padding: 10
        }
      },
      elements: {
        geoFeature: {
          borderColor: 'rgba(255, 255, 255, 0.2)',
          borderWidth: 1.1,
          hoverBorderColor: 'rgba(255, 255, 255, 0.85)',
          hoverBorderWidth: 1.6,
          graticuleBorderColor: 'rgba(255, 255, 255, 0.08)',
          graticuleBorderWidth: 0.6
        }
      },
      showOutline: true,
      showGraticule: false,
      scales: {
        projection: {
          axis: 'x',
          projection: 'mercator'
        },
        color: {
          axis: 'x',
          min: 0,
          max,
          quantize: 8,
          missing: '#0c1424',
          display: false,
          ticks: { display: false },
          interpolate: (value: number) => this.interpolateColor(value)
        }
      }
    };
  }

  private interpolateColor(value: number): string {
    const clamped = Math.min(1, Math.max(0, value));
    const start = [9, 12, 24];
    const end = [180, 225, 255];
    const channel = (index: number) => Math.round(start[index] + (end[index] - start[index]) * clamped);
    const alpha = 0.55 + clamped * 0.4;
    return `rgba(${channel(0)}, ${channel(1)}, ${channel(2)}, ${alpha})`;
  }
}
