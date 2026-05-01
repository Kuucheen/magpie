import {Component, EventEmitter, Input, Output} from '@angular/core';
import {FormsModule} from '@angular/forms';
import {RadioButtonModule} from 'primeng/radiobutton';

export type BulkActionScope = 'all' | 'selected';

@Component({
  selector: 'app-bulk-scope-selector',
  standalone: true,
  imports: [FormsModule, RadioButtonModule],
  templateUrl: './bulk-scope-selector.component.html',
  styleUrl: './bulk-scope-selector.component.scss',
})
export class BulkScopeSelectorComponent {
  @Input() scope: BulkActionScope = 'all';
  @Input() radioName = 'bulkActionScope';
  @Input() allInputId = 'bulkActionAll';
  @Input() selectedInputId = 'bulkActionSelected';
  @Input() allTitle = 'All';
  @Input() allDescription = '';
  @Input() selectedTitle = 'Selected';
  @Input() selectedDescription = '';
  @Input() selectedDisabled = false;
  @Input() accentColor = '#1cbb57';

  @Output() scopeChange = new EventEmitter<BulkActionScope>();

  setScope(value: string): void {
    if (value !== 'all' && value !== 'selected') {
      return;
    }

    if (value === 'selected' && this.selectedDisabled) {
      return;
    }

    this.scope = value;
    this.scopeChange.emit(value);
  }
}
