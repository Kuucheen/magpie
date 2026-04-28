import {Component, EventEmitter, Input, Output} from '@angular/core';
import {FormGroup, ReactiveFormsModule} from '@angular/forms';
import {NgClass} from '@angular/common';
import {ButtonModule} from 'primeng/button';
import {CheckboxModule} from 'primeng/checkbox';
import {InputNumberModule} from 'primeng/inputnumber';
import {Select} from 'primeng/select';

@Component({
  selector: 'app-scrape-source-filter-panel',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    NgClass,
    ButtonModule,
    CheckboxModule,
    InputNumberModule,
    Select,
  ],
  templateUrl: './scrape-source-filter-panel.component.html',
  styleUrl: './scrape-source-filter-panel.component.scss',
})
export class ScrapeSourceFilterPanelComponent {
  @Input({required: true}) form!: FormGroup;
  @Input() floating = true;

  @Output() apply = new EventEmitter<void>();
  @Output() clear = new EventEmitter<void>();

  readonly countOperatorOptions = [
    {label: '>', value: '>'},
    {label: '<', value: '<'},
  ];
}
