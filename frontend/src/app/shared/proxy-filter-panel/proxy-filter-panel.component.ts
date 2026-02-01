import {Component, EventEmitter, Input, Output} from '@angular/core';
import {FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {ButtonModule} from 'primeng/button';
import {CheckboxModule} from 'primeng/checkbox';
import {InputNumberModule} from 'primeng/inputnumber';
import {Select} from 'primeng/select';
import {MultiSelectModule} from 'primeng/multiselect';
import {NgClass} from '@angular/common';

export type ProxyFilterOption = {
  label: string;
  value: string;
};

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

  @Output() apply = new EventEmitter<void>();
  @Output() clear = new EventEmitter<void>();
}
