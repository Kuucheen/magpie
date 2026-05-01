import {Component, Input, OnChanges, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule, Validators} from '@angular/forms';
import {Button} from 'primeng/button';
import {InputTextModule} from 'primeng/inputtext';
import {DialogModule} from 'primeng/dialog';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';
import {ScrapeSourceInfo} from '../../../models/ScrapeSourceInfo';
import {ScrapeSourceExportSettings} from '../../../models/ScrapeSourceExportSettings';
import {TooltipComponent} from '../../../tooltip/tooltip.component';
import {ScrapeSourceFilterPanelComponent} from '../scrape-source-filter-panel/scrape-source-filter-panel.component';
import {BulkScopeSelectorComponent} from '../../../shared/bulk-scope-selector/bulk-scope-selector.component';
import {buildDatedExportFileName, downloadTextFile, extractHttpErrorMessage} from '../../../shared/export-file-utils';
import {normalizeNumber} from '../../../shared/number-utils';

type ExportSourcesFormDefaults = {
  output: string;
  filter: boolean;
  http: boolean;
  https: boolean;
  proxyCountOperator: '<' | '>';
  proxyCount: number;
  aliveCountOperator: '<' | '>';
  aliveCount: number;
};

@Component({
  selector: 'app-export-sources',
  standalone: true,
  imports: [
    FormsModule,
    ReactiveFormsModule,
    Button,
    InputTextModule,
    DialogModule,
    CheckboxComponent,
    TooltipComponent,
    ScrapeSourceFilterPanelComponent,
    BulkScopeSelectorComponent,
  ],
  templateUrl: './export-sources.component.html',
  styleUrls: ['./export-sources.component.scss'],
})
export class ExportSourcesComponent implements OnChanges {
  @Input() selectedSources: ScrapeSourceInfo[] = [];
  @Input() allSources: ScrapeSourceInfo[] = [];

  dialogVisible = false;
  isExporting = false;
  exportOption: 'all' | 'selected' = 'all';
  exportForm: FormGroup;

  readonly predefinedFilters: string[] = ['protocol', 'url', 'proxy_count', 'alive_proxy_count'];
  private readonly defaultFormValues: ExportSourcesFormDefaults = {
    output: 'protocol://url',
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
    this.exportForm = this.fb.group({
      output: [this.defaultFormValues.output, Validators.required],
      filter: [this.defaultFormValues.filter],
      http: [this.defaultFormValues.http],
      https: [this.defaultFormValues.https],
      proxyCountOperator: [this.defaultFormValues.proxyCountOperator, Validators.required],
      proxyCount: [this.defaultFormValues.proxyCount, Validators.required],
      aliveCountOperator: [this.defaultFormValues.aliveCountOperator, Validators.required],
      aliveCount: [this.defaultFormValues.aliveCount, Validators.required],
    });
  }

  clearExportFilters(): void {
    this.exportForm.patchValue({
      http: false,
      https: false,
      proxyCountOperator: this.defaultFormValues.proxyCountOperator,
      proxyCount: 0,
      aliveCountOperator: this.defaultFormValues.aliveCountOperator,
      aliveCount: 0,
    });
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['selectedSources'] && this.exportOption === 'selected' && !this.canExportSelected()) {
      this.exportOption = 'all';
    }
  }

  openDialog(): void {
    if (!this.hasAnySources()) {
      this.notification.showError('No scrape sources available to export.');
      return;
    }

    this.exportOption = this.canExportSelected() ? 'selected' : 'all';
    this.dialogVisible = true;
  }

  closeDialog(): void {
    this.dialogVisible = false;
  }

  onDialogHide(): void {
    this.resetFormState();
  }

  hasAnySources(): boolean {
    return (this.allSources?.length ?? 0) > 0;
  }

  canExportSelected(): boolean {
    return (this.selectedSources?.length ?? 0) > 0;
  }

  addToFilter(text: string): void {
    const currentValue = this.exportForm.get('output')?.value;
    const newValue = currentValue && currentValue !== '' ? `${currentValue};${text}` : text;
    this.exportForm.get('output')?.setValue(newValue);
  }

  submitExport(): void {
    const sources = this.exportOption === 'selected' ? this.selectedSources : this.allSources;
    if (!sources || sources.length === 0) {
      this.notification.showError('No scrape sources selected for export.');
      return;
    }

    this.isExporting = true;
    const exportSettings = this.transformFormToExport(this.exportForm, sources, this.exportOption);
    const fileName = this.buildFileName();

    this.http.exportScrapeSources(exportSettings).subscribe({
      next: res => {
        downloadTextFile(res, fileName);
        this.isExporting = false;
        this.closeDialog();
      },
      error: err => {
        this.isExporting = false;
        const message = extractHttpErrorMessage(err);
        this.notification.showError('Error while exporting scrape sources: ' + message);
      }
    });
  }

  private resetFormState(): void {
    this.exportForm.reset(this.defaultFormValues);
    this.exportOption = 'all';
    this.isExporting = false;
  }

  private transformFormToExport(exportForm: FormGroup, sources: ScrapeSourceInfo[], scope: 'all' | 'selected'): ScrapeSourceExportSettings {
    const formValue = exportForm.getRawValue();
    const sourceIds = scope === 'selected' ? sources.map(source => source.id) : [];
    const filtersEnabled = Boolean(formValue.filter);

    return {
      scrapeSources: sourceIds,
      filter: filtersEnabled,
      http: filtersEnabled ? Boolean(formValue.http) : false,
      https: filtersEnabled ? Boolean(formValue.https) : false,
      proxyCountOperator: formValue.proxyCountOperator === '<' ? '<' : '>',
      proxyCount: filtersEnabled ? normalizeNumber(formValue.proxyCount) : 0,
      aliveCountOperator: formValue.aliveCountOperator === '<' ? '<' : '>',
      aliveCount: filtersEnabled ? normalizeNumber(formValue.aliveCount) : 0,
      outputFormat: formValue.output,
    };
  }

  private buildFileName(): string {
    return buildDatedExportFileName('magpie-sources.txt');
  }
}
