import {Component, EventEmitter, Input, OnChanges, Output, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {Button} from 'primeng/button';
import {RadioButtonModule} from 'primeng/radiobutton';
import {DialogModule} from 'primeng/dialog';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';
import {ScrapeSourceInfo} from '../../../models/ScrapeSourceInfo';
import {ScrapeSourceDeleteSettings} from '../../../models/ScrapeSourceDeleteSettings';
import {TooltipComponent} from '../../../tooltip/tooltip.component';
import {ScrapeSourceFilterPanelComponent} from '../scrape-source-filter-panel/scrape-source-filter-panel.component';

type DeleteSourcesFormDefaults = {
  filter: boolean;
  http: boolean;
  https: boolean;
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
    DialogModule,
    CheckboxComponent,
    TooltipComponent,
    ScrapeSourceFilterPanelComponent,
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

  private readonly defaultFormValues: DeleteSourcesFormDefaults = {
    filter: false,
    http: true,
    https: true,
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
      http: [this.defaultFormValues.http],
      https: [this.defaultFormValues.https],
      proxyCountOperator: [this.defaultFormValues.proxyCountOperator],
      proxyCount: [this.defaultFormValues.proxyCount],
      aliveCountOperator: [this.defaultFormValues.aliveCountOperator],
      aliveCount: [this.defaultFormValues.aliveCount],
    });
  }

  clearDeleteFilters(): void {
    this.deleteForm.patchValue({
      http: false,
      https: false,
      proxyCountOperator: this.defaultFormValues.proxyCountOperator,
      proxyCount: 0,
      aliveCountOperator: this.defaultFormValues.aliveCountOperator,
      aliveCount: 0,
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
    const filtersEnabled = Boolean(formValue.filter);

    return {
      scrapeSources: sources.map(source => source.id),
      filter: filtersEnabled,
      http: filtersEnabled ? Boolean(formValue.http) : false,
      https: filtersEnabled ? Boolean(formValue.https) : false,
      proxyCountOperator: formValue.proxyCountOperator === '<' ? '<' : '>',
      proxyCount: filtersEnabled ? this.normalizeCount(formValue.proxyCount) : 0,
      aliveCountOperator: formValue.aliveCountOperator === '<' ? '<' : '>',
      aliveCount: filtersEnabled ? this.normalizeCount(formValue.aliveCount) : 0,
      scope,
    };
  }

  private normalizeCount(value: number | string | null | undefined): number {
    if (value === null || value === undefined) {
      return 0;
    }
    const parsed = typeof value === 'string' ? Number(value) : value;
    if (!Number.isFinite(parsed)) {
      return 0;
    }
    return Math.max(0, Math.floor(parsed));
  }
}
