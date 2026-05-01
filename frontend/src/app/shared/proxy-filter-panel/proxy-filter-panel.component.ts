import {Component, EventEmitter, Input, Output, signal} from '@angular/core';
import {FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {ButtonModule} from 'primeng/button';
import {CheckboxModule} from 'primeng/checkbox';
import {InputNumberModule} from 'primeng/inputnumber';
import {Select} from 'primeng/select';
import {MultiSelectModule} from 'primeng/multiselect';
import {NgClass} from '@angular/common';
import {ProxyFilterOption} from '../proxy-filters';

@Component({
  selector: 'app-proxy-filter-panel',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    FormsModule,
    ButtonModule,
    CheckboxModule,
    InputNumberModule,
    Select,
    MultiSelectModule,
    NgClass,
  ],
  templateUrl: './proxy-filter-panel.component.html',
  styleUrl: './proxy-filter-panel.component.scss',
})
export class ProxyFilterPanelComponent {
  @Input({required: true}) form!: FormGroup;
  @Input() countryOptions: ProxyFilterOption[] = [];
  @Input() typeOptions: ProxyFilterOption[] = [];
  @Input() anonymityOptions: ProxyFilterOption[] = [];
  @Input() proxyStatusOptions: ProxyFilterOption[] = [];
  @Input() proxyReputationOptions: ProxyFilterOption[] = [];
  @Input() floating = true;
  @Input() showHeader = true;

  @Output() apply = new EventEmitter<void>();
  @Output() clear = new EventEmitter<void>();

  protected readonly healthFiltersExpanded = signal(false);

  toggleHealthFilters(): void {
    this.healthFiltersExpanded.update(value => !value);
  }

  activeHealthFilterCount(): number {
    if (!this.form) {
      return 0;
    }

    const value = this.form.getRawValue() as Record<string, unknown>;
    const healthKeys = [
      'minHealthOverall',
      'minHealthHttp',
      'minHealthHttps',
      'minHealthSocks4',
      'minHealthSocks5',
    ];

    return healthKeys.reduce((count, key) => {
      const current = Number(value[key] ?? 0);
      return Number.isFinite(current) && current > 0 ? count + 1 : count;
    }, 0);
  }
}
