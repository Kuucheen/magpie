import {Component, OnDestroy, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, ReactiveFormsModule, Validators} from '@angular/forms';
import {CheckboxComponent} from '../../checkbox/checkbox.component';
import {InputText} from 'primeng/inputtext';
import {Button} from 'primeng/button';
import {Select} from 'primeng/select';
import {Chip} from 'primeng/chip';
import {PrimeTemplate} from 'primeng/api';
import {SettingsService} from '../../services/settings.service';
import {NotificationService} from '../../services/notification-service.service';
import {UserSettings} from '../../models/UserSettings';
import {Subject} from 'rxjs';
import {filter, takeUntil} from 'rxjs/operators';
import {TooltipComponent} from '../../tooltip/tooltip.component';

@Component({
  selector: 'app-checker-settings',
  standalone: true,
  imports: [ReactiveFormsModule, CheckboxComponent, InputText, Button, Select, Chip, PrimeTemplate, TooltipComponent],
  templateUrl: './checker-settings.component.html',
  styleUrls: ['./checker-settings.component.scss']
})
export class CheckerSettingsComponent implements OnInit, OnDestroy {
  settingsForm: FormGroup;
  transportProtocolOptions = [
    { label: 'TCP', value: 'tcp' },
    { label: 'QUIC', value: 'quic' },
    { label: 'HTTP/3', value: 'http3' },
  ];
  readonly transportProtocolTooltip =
    'TCP uses standard HTTP over TCP. QUIC and HTTP/3 both use HTTP/3 over QUIC; QUIC enables HTTP/3 datagrams (unreliable messages), HTTP/3 uses streams only.';
  private destroy$ = new Subject<void>();

  constructor(private fb: FormBuilder, private settingsService: SettingsService) {
    this.settingsForm = this.createForm();
    this.configureAutoRemoveThresholdToggle();
  }

  ngOnInit(): void {
    this.populateForm(this.settingsService.getUserSettings());

    this.settingsService.userSettings$
      .pipe(
        filter((settings): settings is UserSettings => !!settings),
        takeUntil(this.destroy$)
      )
      .subscribe(settings => this.populateForm(settings));
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  private createForm(): FormGroup {
    return this.fb.group({
      HTTPProtocol: [false],
      HTTPSProtocol: [true],
      SOCKS4Protocol: [false],
      SOCKS5Protocol: [false],
      Timeout: [7500],
      Retries: [2],
      UseHttpsForSocks: [true],
      TransportProtocol: ['tcp'],
      AutoRemoveFailingProxies: [false],
      AutoRemoveFailureThreshold: [3, [Validators.min(1), Validators.max(255)]],
    });
  }

  private populateForm(settings: UserSettings | undefined): void {
    if (!settings) {
      return;
    }

    const transportValues = this.transportProtocolOptions.map(option => option.value);
    const selectedTransport = transportValues.includes(settings.transport_protocol)
      ? settings.transport_protocol
      : 'tcp';

    this.settingsForm.patchValue({
      HTTPProtocol: settings.http_protocol,
      HTTPSProtocol: settings.https_protocol,
      SOCKS4Protocol: settings.socks4_protocol,
      SOCKS5Protocol: settings.socks5_protocol,
      Timeout: settings.timeout,
      Retries: settings.retries,
      UseHttpsForSocks: settings.UseHttpsForSocks,
      TransportProtocol: selectedTransport,
      AutoRemoveFailingProxies: settings.auto_remove_failing_proxies,
      AutoRemoveFailureThreshold: settings.auto_remove_failure_threshold,
    });

    this.settingsForm.markAsPristine();
  }

  onSubmit(): void {
    const current = this.settingsService.getUserSettings();
    const payload = {
      ...this.settingsForm.getRawValue(),
      judges: current?.judges ?? [],
    };

    const threshold = Number(payload.AutoRemoveFailureThreshold ?? 1);
    const normalizedThreshold = Math.round(Number.isFinite(threshold) ? threshold : 1);
    payload.AutoRemoveFailureThreshold = Math.min(Math.max(normalizedThreshold, 1), 255);

    this.settingsService.saveUserSettings(payload).subscribe({
      next: (resp) => {
        NotificationService.showSuccess(resp.message);
        this.populateForm(this.settingsService.getUserSettings());
      },
      error: (err) => {
        console.error('Error saving settings:', err);
        NotificationService.showError('Failed to save settings!');
      }
    });
  }

  private configureAutoRemoveThresholdToggle(): void {
    const autoRemoveControl = this.settingsForm.get('AutoRemoveFailingProxies');
    const thresholdControl = this.settingsForm.get('AutoRemoveFailureThreshold');

    if (!autoRemoveControl || !thresholdControl) {
      return;
    }

    const syncThresholdState = (isEnabled: boolean): void => {
      if (isEnabled) {
        thresholdControl.enable({emitEvent: false});
      } else {
        thresholdControl.disable({emitEvent: false});
      }
    };

    syncThresholdState(!!autoRemoveControl.value);

    autoRemoveControl.valueChanges
      .pipe(takeUntil(this.destroy$))
      .subscribe(value => syncThresholdState(!!value));
  }
}
