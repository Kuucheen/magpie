
import {Component, Input, OnChanges, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule, Validators} from '@angular/forms';
import {Button} from 'primeng/button';
import {RadioButtonModule} from 'primeng/radiobutton';
import {InputNumberModule} from 'primeng/inputnumber';
import {InputTextModule} from 'primeng/inputtext';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {SettingsService} from '../../../services/settings.service';
import {HttpService} from '../../../services/http.service';
import {ProxyInfo} from '../../../models/ProxyInfo';
import {ExportSettings} from '../../../models/ExportSettings';
import {DialogModule} from 'primeng/dialog';
import {Select} from 'primeng/select';
import {MultiSelectModule} from 'primeng/multiselect';
import {NotificationService} from '../../../services/notification-service.service';
import {TooltipComponent} from '../../../tooltip/tooltip.component';

type ExportFormDefaults = {
  output: string;
  filter: boolean;
  HTTPProtocol: boolean;
  HTTPSProtocol: boolean;
  SOCKS4Protocol: boolean;
  SOCKS5Protocol: boolean;
  Retries: number;
  Timeout: number;
  proxyStatus: 'all' | 'alive' | 'dead';
  proxyReputations: string[];
};

@Component({
  selector: 'app-export-proxies',
  standalone: true,
  imports: [
    FormsModule,
    ReactiveFormsModule,
    Button,
    RadioButtonModule,
    InputNumberModule,
    InputTextModule,
    CheckboxComponent,
    DialogModule,
    Select,
    MultiSelectModule,
    TooltipComponent
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
  readonly proxyStatusOptions = [
    {label: 'All Proxies', value: 'all'},
    {label: 'Only Alive Proxies', value: 'alive'},
    {label: 'Only Dead Proxies', value: 'dead'},
  ];
  readonly proxyReputationOptions = [
    {label: 'Good', value: 'good'},
    {label: 'Neutral', value: 'neutral'},
    {label: 'Poor', value: 'poor'},
    {label: 'Unknown', value: 'unknown'},
  ];

  private defaultFormValues: ExportFormDefaults;

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private http: HttpService,
    private notification: NotificationService
  ) {
    const settings = this.settingsService.getUserSettings();

    this.defaultFormValues = {
      output: 'protocol://ip:port',
      filter: false,
      HTTPProtocol: settings?.http_protocol ?? false,
      HTTPSProtocol: settings?.https_protocol ?? false,
      SOCKS4Protocol: settings?.socks4_protocol ?? false,
      SOCKS5Protocol: settings?.socks5_protocol ?? false,
      Retries: settings?.retries ?? 0,
      Timeout: settings?.timeout ?? 0,
      proxyStatus: 'all',
      proxyReputations: [],
    };

    this.exportForm = this.fb.group({
      output: [this.defaultFormValues.output, Validators.required],
      filter: [this.defaultFormValues.filter],
      HTTPProtocol: [this.defaultFormValues.HTTPProtocol],
      HTTPSProtocol: [this.defaultFormValues.HTTPSProtocol],
      SOCKS4Protocol: [this.defaultFormValues.SOCKS4Protocol],
      SOCKS5Protocol: [this.defaultFormValues.SOCKS5Protocol],
      Retries: [this.defaultFormValues.Retries, Validators.required],
      Timeout: [this.defaultFormValues.Timeout, Validators.required],
      proxyStatus: [this.defaultFormValues.proxyStatus],
      proxyReputations: [this.defaultFormValues.proxyReputations],
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
        const message = err?.error?.message ?? err?.message ?? 'Unknown error';
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
      HTTPProtocol: settings.http_protocol,
      HTTPSProtocol: settings.https_protocol,
      SOCKS4Protocol: settings.socks4_protocol,
      SOCKS5Protocol: settings.socks5_protocol,
      Retries: settings.retries,
      Timeout: settings.timeout,
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
    const reputationSelection = this.normalizeReputationSelection(formValue.proxyReputations);

    return {
      proxies: proxyIds,
      filter: formValue.filter,
      http: formValue.HTTPProtocol,
      https: formValue.HTTPSProtocol,
      socks4: formValue.SOCKS4Protocol,
      socks5: formValue.SOCKS5Protocol,
      maxRetries: formValue.Retries,
      maxTimeout: formValue.Timeout,
      proxyStatus: formValue.proxyStatus,
      reputationLabels: reputationSelection,
      outputFormat: formValue.output
    };
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

  private normalizeReputationSelection(rawValue: unknown): string[] {
    if (Array.isArray(rawValue)) {
      return rawValue.filter((value): value is string => typeof value === 'string' && value.trim().length > 0);
    }
    if (typeof rawValue === 'string' && rawValue.trim().length > 0) {
      return [rawValue.trim()];
    }
    return [];
  }
}
