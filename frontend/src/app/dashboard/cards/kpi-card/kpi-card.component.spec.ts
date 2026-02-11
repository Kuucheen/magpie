import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LOCALE_ID, SimpleChange } from '@angular/core';

import { KpiCardComponent } from './kpi-card.component';

describe('KpiCardComponent', () => {
  let component: KpiCardComponent;
  let fixture: ComponentFixture<KpiCardComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [KpiCardComponent],
      providers: [{ provide: LOCALE_ID, useValue: 'de-DE' }]
    })
      .compileComponents();

    fixture = TestBed.createComponent(KpiCardComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should align chart data with current value', () => {
    component.value = 200;
    component.chartValues = [120, 140, 160, 180];

    component.ngOnChanges({
      value: new SimpleChange(null, component.value, true),
      chartValues: new SimpleChange(null, component.chartValues, true)
    });

    const dataset = component.sparklineData.datasets[0].data as number[];
    expect(dataset.length).toBe(5);
    expect(dataset[dataset.length - 1]).toBe(200);
    expect(component.resolvedChange).toBe(11.1);
  });

  it('should allow explicit change override', () => {
    component.value = 150;
    component.chartValues = [100, 110, 120, 130];
    component.change = -2.5;

    component.ngOnChanges({
      change: new SimpleChange(null, component.change, true),
      value: new SimpleChange(null, component.value, true),
      chartValues: new SimpleChange(null, component.chartValues, true)
    });

    expect(component.resolvedChange).toBe(-2.5);
  });

  it('should calculate change from numeric value even when displayValue is localized', () => {
    component.value = 25000;
    component.displayValue = '25.000';
    component.chartValues = [20000, 22000, 24000];

    component.ngOnChanges({
      value: new SimpleChange(null, component.value, true),
      displayValue: new SimpleChange(null, component.displayValue, true),
      chartValues: new SimpleChange(null, component.chartValues, true)
    });

    expect(component.resolvedChange).toBe(4.2);
    const dataset = component.sparklineData.datasets[0].data as number[];
    expect(dataset[dataset.length - 1]).toBe(25000);
  });

  it('should configure tooltip to show only value', () => {
    const tooltipConfig = component.sparklineOptions.plugins.tooltip;
    const label = tooltipConfig.callbacks.label({ parsed: { y: 1000 } } as any);
    const title = tooltipConfig.callbacks.title();

    expect(label).toBe('1.000');
    expect(title).toEqual([]);
  });

  it('should localize percent change label', () => {
    component.value = 25000;
    component.chartValues = [20000, 22000, 24000];
    component.ngOnChanges({
      value: new SimpleChange(null, component.value, true),
      chartValues: new SimpleChange(null, component.chartValues, true)
    });

    const formatted = component.formatChange(component.resolvedChange);
    expect(formatted).toContain('4,2');
    expect(formatted).toContain('%');
  });
});
