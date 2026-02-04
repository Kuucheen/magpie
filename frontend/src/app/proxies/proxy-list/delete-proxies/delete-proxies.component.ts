
import {Component, EventEmitter, Input, OnChanges, Output, SimpleChanges} from '@angular/core';
import {FormBuilder, FormGroup, FormsModule, ReactiveFormsModule} from '@angular/forms';
import {Button} from 'primeng/button';
import {RadioButtonModule} from 'primeng/radiobutton';
import {InputNumberModule} from 'primeng/inputnumber';
import {CheckboxComponent} from '../../../checkbox/checkbox.component';
import {SettingsService} from '../../../services/settings.service';
import {HttpService} from '../../../services/http.service';
import {ProxyInfo} from '../../../models/ProxyInfo';
import {DialogModule} from 'primeng/dialog';
import {Select} from 'primeng/select';
import {MultiSelectModule} from 'primeng/multiselect';
import {NotificationService} from '../../../services/notification-service.service';
import {DeleteSettings} from '../../../models/DeleteSettings';
import {TooltipComponent} from '../../../tooltip/tooltip.component';

type DeleteFormDefaults = {
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
  selector: 'app-delete-proxies',
  standalone: true,
  imports: [
    FormsModule,
    ReactiveFormsModule,
    Button,
    RadioButtonModule,
    InputNumberModule,
    CheckboxComponent,
    DialogModule,
    Select,
    MultiSelectModule,
    TooltipComponent
],
  templateUrl: './delete-proxies.component.html',
  styleUrls: ['./delete-proxies.component.scss'],
})
export class DeleteProxiesComponent implements OnChanges {
  @Input() selectedProxies: ProxyInfo[] = [];
  @Input() allProxies: ProxyInfo[] = [];
  @Output() proxiesDeleted = new EventEmitter<void>();

  dialogVisible = false;
  isDeleting = false;
  deleteOption: 'all' | 'selected' = 'all';
  deleteForm: FormGroup;

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

  private defaultFormValues: DeleteFormDefaults;

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private http: HttpService,
    private notification: NotificationService
  ) {
    const settings = this.settingsService.getUserSettings();

    this.defaultFormValues = {
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

    this.deleteForm = this.fb.group({
      filter: [this.defaultFormValues.filter],
      HTTPProtocol: [this.defaultFormValues.HTTPProtocol],
      HTTPSProtocol: [this.defaultFormValues.HTTPSProtocol],
      SOCKS4Protocol: [this.defaultFormValues.SOCKS4Protocol],
      SOCKS5Protocol: [this.defaultFormValues.SOCKS5Protocol],
      Retries: [this.defaultFormValues.Retries],
      Timeout: [this.defaultFormValues.Timeout],
      proxyStatus: [this.defaultFormValues.proxyStatus],
      proxyReputations: [this.defaultFormValues.proxyReputations],
    });
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['selectedProxies'] && this.deleteOption === 'selected' && !this.canDeleteSelected()) {
      this.deleteOption = 'all';
    }
  }

  openDialog(): void {
    if (!this.hasAnyProxies()) {
      this.notification.showError('No proxies available to delete.');
      return;
    }

    this.syncDefaultsWithUserSettings();
    this.deleteOption = this.canDeleteSelected() ? 'selected' : 'all';
    this.dialogVisible = true;
  }

  closeDialog(): void {
    this.dialogVisible = false;
  }

  onDialogHide(): void {
    this.resetFormState();
  }

  hasAnyProxies(): boolean {
    return (this.allProxies?.length ?? 0) > 0 || (this.selectedProxies?.length ?? 0) > 0;
  }

  canDeleteSelected(): boolean {
    return (this.selectedProxies?.length ?? 0) > 0;
  }

  submitDelete(): void {
    if (this.deleteOption === 'selected' && !this.canDeleteSelected()) {
      this.notification.showError('No proxies selected for deletion.');
      return;
    }

    const deleteSettings = this.transformFormToDelete(this.deleteForm, this.deleteOption);
    if (deleteSettings.scope === 'selected' && deleteSettings.proxies.length === 0) {
      this.notification.showError('No proxies selected for deletion.');
      return;
    }

    this.isDeleting = true;

    this.http.deleteProxies(deleteSettings).subscribe({
      next: res => {
        const message = res;
        const normalized = message.trim().toLowerCase();

        if (normalized.includes('no proxies')) {
          this.notification.showInfo(message);
        } else {
          this.notification.showSuccess(message);
        }

        this.isDeleting = false;
        this.closeDialog();
        this.proxiesDeleted.emit();
      },
      error: err => {
        this.isDeleting = false;
        const message = err?.error?.message ?? err?.message ?? 'Unknown error';
        this.notification.showError('Could not delete proxies: ' + message);
      }
    });
  }

  private resetFormState(): void {
    this.deleteForm.reset(this.defaultFormValues);
    this.deleteOption = 'all';
    this.isDeleting = false;
  }

  private syncDefaultsWithUserSettings(): void {
    const settings = this.settingsService.getUserSettings();
    if (!settings) {
      return;
    }

    const updatedDefaults: Partial<DeleteFormDefaults> = {
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

    this.deleteForm.patchValue(updatedDefaults, {emitEvent: false});
  }

  private transformFormToDelete(form: FormGroup, scope: 'all' | 'selected'): DeleteSettings {
    const formValue = form.value;
    const proxies = scope === 'selected' ? this.selectedProxies : [];
    const reputationSelection = this.normalizeReputationSelection(formValue.proxyReputations);

    return {
      proxies: proxies.map(proxy => proxy.id),
      filter: formValue.filter,
      http: formValue.HTTPProtocol,
      https: formValue.HTTPSProtocol,
      socks4: formValue.SOCKS4Protocol,
      socks5: formValue.SOCKS5Protocol,
      maxRetries: formValue.Retries,
      maxTimeout: formValue.Timeout,
      proxyStatus: formValue.proxyStatus,
      reputationLabels: reputationSelection,
      scope,
    };
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
