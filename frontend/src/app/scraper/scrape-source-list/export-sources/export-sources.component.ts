import {Component, Input, OnChanges, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule, Validators} from '@angular/forms';
import {Button} from 'primeng/button';
import {RadioButtonModule} from 'primeng/radiobutton';
import {InputNumberModule} from 'primeng/inputnumber';
import {InputTextModule} from 'primeng/inputtext';
import {DialogModule} from 'primeng/dialog';
import {Select} from 'primeng/select';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';
import {ScrapeSourceInfo} from '../../../models/ScrapeSourceInfo';
import {ScrapeSourceExportSettings} from '../../../models/ScrapeSourceExportSettings';
import {TooltipComponent} from '../../../tooltip/tooltip.component';

type ExportSourcesFormDefaults = {
  output: string;
  filter: boolean;
  HTTPProtocol: boolean;
  HTTPSProtocol: boolean;
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
    RadioButtonModule,
    InputNumberModule,
    InputTextModule,
    DialogModule,
    Select,
    CheckboxComponent,
    TooltipComponent,
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
  readonly countOperatorOptions = [
    {label: '>', value: '>'},
    {label: '<', value: '<'},
  ];

  private readonly defaultFormValues: ExportSourcesFormDefaults = {
    output: 'protocol://url',
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
    this.exportForm = this.fb.group({
      output: [this.defaultFormValues.output, Validators.required],
      filter: [this.defaultFormValues.filter],
      HTTPProtocol: [this.defaultFormValues.HTTPProtocol],
      HTTPSProtocol: [this.defaultFormValues.HTTPSProtocol],
      proxyCountOperator: [this.defaultFormValues.proxyCountOperator, Validators.required],
      proxyCount: [this.defaultFormValues.proxyCount, Validators.required],
      aliveCountOperator: [this.defaultFormValues.aliveCountOperator, Validators.required],
      aliveCount: [this.defaultFormValues.aliveCount, Validators.required],
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
        this.downloadFile(res, fileName);
        this.isExporting = false;
        this.closeDialog();
      },
      error: err => {
        this.isExporting = false;
        const message = this.extractExportErrorMessage(err);
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

    return {
      scrapeSources: sourceIds,
      filter: formValue.filter,
      http: formValue.HTTPProtocol,
      https: formValue.HTTPSProtocol,
      proxyCountOperator: formValue.proxyCountOperator === '<' ? '<' : '>',
      proxyCount: formValue.proxyCount,
      aliveCountOperator: formValue.aliveCountOperator === '<' ? '<' : '>',
      aliveCount: formValue.aliveCount,
      outputFormat: formValue.output,
    };
  }

  private buildFileName(): string {
    const today = new Date();
    const year = today.getFullYear();
    const month = String(today.getMonth() + 1).padStart(2, '0');
    const day = String(today.getDate()).padStart(2, '0');
    const randomCode = this.generateRandomCode(4);
    return `${year}-${month}-${day}-${randomCode}-magpie-sources.txt`;
  }

  private generateRandomCode(length: number = 4): string {
    const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    let result = '';
    for (let i = 0; i < length; i++) {
      result += characters.charAt(Math.floor(Math.random() * characters.length));
    }
    return result;
  }

  private downloadFile(data: BlobPart, fileName: string): void {
    const blob = new Blob([data], {type: 'text/plain'});
    const url = window.URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = fileName;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
    window.URL.revokeObjectURL(url);
  }

  private extractExportErrorMessage(error: unknown): string {
    if (typeof error === 'object' && error !== null) {
      const httpError = error as {error?: unknown; message?: unknown};

      if (typeof httpError.error === 'string' && httpError.error.trim().length > 0) {
        try {
          const parsed = JSON.parse(httpError.error) as {error?: unknown; message?: unknown};
          if (typeof parsed.error === 'string' && parsed.error.trim().length > 0) {
            return parsed.error;
          }
          if (typeof parsed.message === 'string' && parsed.message.trim().length > 0) {
            return parsed.message;
          }
        } catch {
          return httpError.error;
        }
      }

      if (typeof httpError.error === 'object' && httpError.error !== null) {
        const payload = httpError.error as {error?: unknown; message?: unknown};
        if (typeof payload.error === 'string' && payload.error.trim().length > 0) {
          return payload.error;
        }
        if (typeof payload.message === 'string' && payload.message.trim().length > 0) {
          return payload.message;
        }
      }

      if (typeof httpError.message === 'string' && httpError.message.trim().length > 0) {
        return httpError.message;
      }
    }

    return 'Unknown error';
  }
}
