
import {Component, Input, OnChanges, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule, Validators} from '@angular/forms';
import {Button} from 'primeng/button';
import {RadioButtonModule} from 'primeng/radiobutton';
import {InputTextModule} from 'primeng/inputtext';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {SettingsService} from '../../../services/settings.service';
import {HttpService} from '../../../services/http.service';
import {ProxyInfo} from '../../../models/ProxyInfo';
import {ExportSettings} from '../../../models/ExportSettings';
import {DialogModule} from 'primeng/dialog';
import {NotificationService} from '../../../services/notification-service.service';
import {TooltipComponent} from '../../../tooltip/tooltip.component';
import {ProxyFilterPanelComponent} from '../../../shared/proxy-filter-panel/proxy-filter-panel.component';
import {
  PROXY_REPUTATION_OPTIONS,
  PROXY_STATUS_OPTIONS,
  ProxyFilterOption,
  ProxyListFilterFormValues,
  buildFilterOptionList,
  createDefaultProxyFilterValues,
  normalizeFilterOptions,
  normalizeNumber,
  normalizePercentage,
  normalizeSelection,
} from '../../../shared/proxy-filters';

type ExportFormDefaults = {
  output: string;
  filter: boolean;
} & ProxyListFilterFormValues;

@Component({
  selector: 'app-export-proxies',
  standalone: true,
  imports: [
    FormsModule,
    ReactiveFormsModule,
    Button,
    RadioButtonModule,
    InputTextModule,
    CheckboxComponent,
    DialogModule,
    TooltipComponent,
    ProxyFilterPanelComponent
  ],
  templateUrl: './export-proxies.component.html',
  styleUrls: ['./export-proxies.component.scss'],
})
export class ExportProxiesComponent implements OnChanges {
  @Input() selectedProxies: ProxyInfo[] = [];
  @Input() allProxies: ProxyInfo[] = [];
  dialogVisible = false;
  isExporting = false;
  exportOption: 'all' | 'selected' = 'all';
  exportForm: FormGroup;

  readonly predefinedFilters: string[] = ['protocol', 'ip', 'port', 'username', 'password', 'country', 'alive', 'type', 'time', 'reputation_label', 'reputation_score'];
  readonly proxyStatusOptions = PROXY_STATUS_OPTIONS;
  readonly proxyReputationOptions = PROXY_REPUTATION_OPTIONS;
  countryOptions: ProxyFilterOption[] = [];
  typeOptions: ProxyFilterOption[] = [];
  anonymityOptions: ProxyFilterOption[] = [];

  private defaultFormValues: ExportFormDefaults;
  private filterOptionsLoaded = false;

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private http: HttpService,
    private notification: NotificationService
  ) {
    const settings = this.settingsService.getUserSettings();

    const defaultFilterValues = createDefaultProxyFilterValues();

    this.defaultFormValues = {
      ...defaultFilterValues,
      output: 'protocol://ip:port',
      filter: false,
      http: settings?.http_protocol ?? false,
      https: settings?.https_protocol ?? false,
      socks4: settings?.socks4_protocol ?? false,
      socks5: settings?.socks5_protocol ?? false,
      maxRetries: settings?.retries ?? 0,
      maxTimeout: settings?.timeout ?? 0,
    };

    this.exportForm = this.fb.group({
      output: [this.defaultFormValues.output, Validators.required],
      filter: [this.defaultFormValues.filter],
      proxyStatus: [this.defaultFormValues.proxyStatus],
      http: [this.defaultFormValues.http],
      https: [this.defaultFormValues.https],
      socks4: [this.defaultFormValues.socks4],
      socks5: [this.defaultFormValues.socks5],
      minHealthOverall: [this.defaultFormValues.minHealthOverall],
      minHealthHttp: [this.defaultFormValues.minHealthHttp],
      minHealthHttps: [this.defaultFormValues.minHealthHttps],
      minHealthSocks4: [this.defaultFormValues.minHealthSocks4],
      minHealthSocks5: [this.defaultFormValues.minHealthSocks5],
      maxTimeout: [this.defaultFormValues.maxTimeout, Validators.required],
      maxRetries: [this.defaultFormValues.maxRetries, Validators.required],
      countries: [this.defaultFormValues.countries],
      types: [this.defaultFormValues.types],
      anonymityLevels: [this.defaultFormValues.anonymityLevels],
      reputationLabels: [this.defaultFormValues.reputationLabels],
    });
  }

  clearExportFilters(): void {
    this.exportForm.patchValue({
      proxyStatus: this.defaultFormValues.proxyStatus,
      http: false,
      https: false,
      socks4: false,
      socks5: false,
      minHealthOverall: 0,
      minHealthHttp: 0,
      minHealthHttps: 0,
      minHealthSocks4: 0,
      minHealthSocks5: 0,
      maxTimeout: 0,
      maxRetries: 0,
      countries: [],
      types: [],
      anonymityLevels: [],
      reputationLabels: [],
    });
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['selectedProxies'] && this.exportOption === 'selected' && !this.canExportSelected()) {
      this.exportOption = 'all';
    }
  }

  openDialog(): void {
    if (!this.hasAnyProxies()) {
      this.notification.showError('No proxies available to export.');
      return;
    }
    this.syncDefaultsWithUserSettings();
    this.ensureFilterOptionsLoaded();
    this.exportOption = this.canExportSelected() ? 'selected' : 'all';
    this.dialogVisible = true;
  }

  closeDialog(): void {
    this.dialogVisible = false;
  }

  onDialogHide(): void {
    this.resetFormState();
  }

  hasAnyProxies(): boolean {
    return (this.allProxies?.length ?? 0) > 0;
  }

  canExportSelected(): boolean {
    return (this.selectedProxies?.length ?? 0) > 0;
  }

  addToFilter(text: string): void {
    const currentValue = this.exportForm.get('output')?.value;
    const newValue = currentValue && currentValue !== '' ? `${currentValue};${text}` : text;
    this.exportForm.get('output')?.setValue(newValue);
  }

  submitExport(): void {
    const proxies = this.exportOption === 'selected' ? this.selectedProxies : this.allProxies;
    if (!proxies || proxies.length === 0) {
      this.notification.showError('No proxies selected for export.');
      return;
    }

    this.isExporting = true;

    const exportSettings = this.transformFormToExport(this.exportForm, proxies, this.exportOption);
    const fileName = this.buildFileName();

    this.http.exportProxies(exportSettings).subscribe({
      next: res => {
        this.downloadFile(res, fileName);
        this.isExporting = false;
        this.closeDialog();
      },
      error: err => {
        this.isExporting = false;
        const message = this.extractExportErrorMessage(err);
        this.notification.showError('Error while exporting proxies: ' + message);
      }
    });
  }

  private resetFormState(): void {
    this.exportForm.reset(this.defaultFormValues);
    this.exportOption = 'all';
    this.isExporting = false;
  }

  private syncDefaultsWithUserSettings(): void {
    const settings = this.settingsService.getUserSettings();
    if (!settings) {
      return;
    }

    const updatedDefaults: Partial<ExportFormDefaults> = {
      http: settings.http_protocol,
      https: settings.https_protocol,
      socks4: settings.socks4_protocol,
      socks5: settings.socks5_protocol,
      maxRetries: settings.retries,
      maxTimeout: settings.timeout,
    };

    this.defaultFormValues = {
      ...this.defaultFormValues,
      ...updatedDefaults,
    };

    this.exportForm.patchValue(updatedDefaults, {emitEvent: false});
  }

  private transformFormToExport(exportForm: FormGroup, proxies: ProxyInfo[], scope: 'all' | 'selected'): ExportSettings {
    const formValue = exportForm.getRawValue();
    const proxyIds = scope === 'selected' ? proxies.map(proxy => proxy.id) : [];
    const filtersEnabled = Boolean(formValue.filter);
    const reputationSelection = filtersEnabled ? normalizeSelection(formValue.reputationLabels) : [];

    return {
      proxies: proxyIds,
      filter: filtersEnabled,
      http: filtersEnabled ? Boolean(formValue.http) : false,
      https: filtersEnabled ? Boolean(formValue.https) : false,
      socks4: filtersEnabled ? Boolean(formValue.socks4) : false,
      socks5: filtersEnabled ? Boolean(formValue.socks5) : false,
      minHealthOverall: filtersEnabled ? normalizePercentage(formValue.minHealthOverall) : 0,
      minHealthHttp: filtersEnabled ? normalizePercentage(formValue.minHealthHttp) : 0,
      minHealthHttps: filtersEnabled ? normalizePercentage(formValue.minHealthHttps) : 0,
      minHealthSocks4: filtersEnabled ? normalizePercentage(formValue.minHealthSocks4) : 0,
      minHealthSocks5: filtersEnabled ? normalizePercentage(formValue.minHealthSocks5) : 0,
      maxRetries: filtersEnabled ? normalizeNumber(formValue.maxRetries) : 0,
      maxTimeout: filtersEnabled ? normalizeNumber(formValue.maxTimeout) : 0,
      countries: filtersEnabled ? normalizeSelection(formValue.countries) : [],
      types: filtersEnabled ? normalizeSelection(formValue.types) : [],
      anonymityLevels: filtersEnabled ? normalizeSelection(formValue.anonymityLevels) : [],
      proxyStatus: filtersEnabled ? (formValue.proxyStatus ?? 'all') : 'all',
      reputationLabels: reputationSelection,
      outputFormat: formValue.output
    };
  }

  private ensureFilterOptionsLoaded(): void {
    if (this.filterOptionsLoaded) {
      return;
    }

    this.http.getProxyFilterOptions().subscribe({
      next: options => {
        const normalized = normalizeFilterOptions(options);
        this.countryOptions = buildFilterOptionList(normalized.countries);
        this.typeOptions = buildFilterOptionList(normalized.types);
        this.anonymityOptions = buildFilterOptionList(normalized.anonymityLevels);
        this.filterOptionsLoaded = true;
      },
      error: err => {
        const message = err?.error?.message ?? err?.message ?? 'Unknown error';
        this.notification.showError('Could not load filter options: ' + message);
      }
    });
  }

  private buildFileName(): string {
    const today = new Date();
    const year = today.getFullYear();
    const month = String(today.getMonth() + 1).padStart(2, '0');
    const day = String(today.getDate()).padStart(2, '0');
    const randomCode = this.generateRandomCode(4);
    return `${year}-${month}-${day}-${randomCode}-magpie.txt`;
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
      const httpError = error as {
        error?: unknown;
        message?: unknown;
      };

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
