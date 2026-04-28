import {Component, EventEmitter, Input, OnChanges, Output, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {Button} from 'primeng/button';
import {RadioButtonModule} from 'primeng/radiobutton';
import {InputNumberModule} from 'primeng/inputnumber';
import {DialogModule} from 'primeng/dialog';
import {Select} from 'primeng/select';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';
import {ScrapeSourceInfo} from '../../../models/ScrapeSourceInfo';
import {ScrapeSourceDeleteSettings} from '../../../models/ScrapeSourceDeleteSettings';
import {TooltipComponent} from '../../../tooltip/tooltip.component';

type DeleteSourcesFormDefaults = {
  filter: boolean;
  HTTPProtocol: boolean;
  HTTPSProtocol: boolean;
  proxyCountOperator: '<' | '>';
  proxyCount: number;
  aliveCountOperator: '<' | '>';
  aliveCount: number;
};

@Component({
  selector: 'app-delete-sources',
  standalone: true,
  imports: [
    FormsModule,
    ReactiveFormsModule,
    Button,
    RadioButtonModule,
    InputNumberModule,
    DialogModule,
    Select,
    CheckboxComponent,
    TooltipComponent,
  ],
  templateUrl: './delete-sources.component.html',
  styleUrls: ['./delete-sources.component.scss'],
})
export class DeleteSourcesComponent implements OnChanges {
  @Input() selectedSources: ScrapeSourceInfo[] = [];
  @Input() allSources: ScrapeSourceInfo[] = [];
  @Output() sourcesDeleted = new EventEmitter<void>();

  dialogVisible = false;
  isDeleting = false;
  deleteOption: 'all' | 'selected' = 'all';
  deleteForm: FormGroup;

  readonly countOperatorOptions = [
    {label: '>', value: '>'},
    {label: '<', value: '<'},
  ];

  private readonly defaultFormValues: DeleteSourcesFormDefaults = {
    filter: false,
    HTTPProtocol: true,
    HTTPSProtocol: true,
    proxyCountOperator: '>',
    proxyCount: 0,
    aliveCountOperator: '>',
    aliveCount: 0,
  };

  constructor(
    private fb: FormBuilder,
    private http: HttpService,
    private notification: NotificationService,
  ) {
    this.deleteForm = this.fb.group({
      filter: [this.defaultFormValues.filter],
      HTTPProtocol: [this.defaultFormValues.HTTPProtocol],
      HTTPSProtocol: [this.defaultFormValues.HTTPSProtocol],
      proxyCountOperator: [this.defaultFormValues.proxyCountOperator],
      proxyCount: [this.defaultFormValues.proxyCount],
      aliveCountOperator: [this.defaultFormValues.aliveCountOperator],
      aliveCount: [this.defaultFormValues.aliveCount],
    });
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['selectedSources'] && this.deleteOption === 'selected' && !this.canDeleteSelected()) {
      this.deleteOption = 'all';
    }
  }

  openDialog(): void {
    if (!this.hasAnySources()) {
      this.notification.showError('No scrape sources available to delete.');
      return;
    }

    this.deleteOption = this.canDeleteSelected() ? 'selected' : 'all';
    this.dialogVisible = true;
  }

  closeDialog(): void {
    this.dialogVisible = false;
  }

  onDialogHide(): void {
    this.resetFormState();
  }

  hasAnySources(): boolean {
    return (this.allSources?.length ?? 0) > 0 || (this.selectedSources?.length ?? 0) > 0;
  }

  canDeleteSelected(): boolean {
    return (this.selectedSources?.length ?? 0) > 0;
  }

  submitDelete(): void {
    if (this.deleteOption === 'selected' && !this.canDeleteSelected()) {
      this.notification.showError('No scrape sources selected for deletion.');
      return;
    }

    const deleteSettings = this.transformFormToDelete(this.deleteForm, this.deleteOption);
    if (deleteSettings.scope === 'selected' && deleteSettings.scrapeSources.length === 0) {
      this.notification.showError('No scrape sources selected for deletion.');
      return;
    }

    this.isDeleting = true;

    this.http.deleteScrapingSource(deleteSettings).subscribe({
      next: res => {
        const message = res;
        const normalized = message.trim().toLowerCase();

        if (normalized.includes('no scraping sources')) {
          this.notification.showInfo(message);
        } else {
          this.notification.showSuccess(message);
        }

        this.isDeleting = false;
        this.closeDialog();
        this.sourcesDeleted.emit();
      },
      error: err => {
        this.isDeleting = false;
        const message = err?.error?.message ?? err?.error?.error ?? err?.message ?? 'Unknown error';
        this.notification.showError('Could not delete scrape sources: ' + message);
      }
    });
  }

  private resetFormState(): void {
    this.deleteForm.reset(this.defaultFormValues);
    this.deleteOption = 'all';
    this.isDeleting = false;
  }

  private transformFormToDelete(form: FormGroup, scope: 'all' | 'selected'): ScrapeSourceDeleteSettings {
    const formValue = form.getRawValue();
    const sources = scope === 'selected' ? this.selectedSources : [];

    return {
      scrapeSources: sources.map(source => source.id),
      filter: formValue.filter,
      http: formValue.HTTPProtocol,
      https: formValue.HTTPSProtocol,
      proxyCountOperator: formValue.proxyCountOperator === '<' ? '<' : '>',
      proxyCount: formValue.proxyCount,
      aliveCountOperator: formValue.aliveCountOperator === '<' ? '<' : '>',
      aliveCount: formValue.aliveCount,
      scope,
    };
  }
}
