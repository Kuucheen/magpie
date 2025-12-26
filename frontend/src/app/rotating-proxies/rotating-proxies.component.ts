import {Component, OnDestroy, OnInit, signal} from '@angular/core';
import {CommonModule, DatePipe} from '@angular/common';
import {FormBuilder, FormGroup, ReactiveFormsModule, Validators} from '@angular/forms';
import {forkJoin, Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';
import {TableModule} from 'primeng/table';
import {ButtonModule} from 'primeng/button';
import {InputTextModule} from 'primeng/inputtext';
import {SelectModule} from 'primeng/select';
import {DialogModule} from 'primeng/dialog';
import {MultiSelectModule} from 'primeng/multiselect';

import {environment} from '../../environments/environment';

import {HttpService} from '../services/http.service';
import {NotificationService} from '../services/notification-service.service';
import {CreateRotatingProxy, RotatingProxy} from '../models/RotatingProxy';
import {UserSettings} from '../models/UserSettings';
import {LoadingComponent} from '../ui-elements/loading/loading.component';
import {TooltipComponent} from '../tooltip/tooltip.component';

@Component({
  selector: 'app-rotating-proxies',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    TableModule,
    ButtonModule,
    InputTextModule,
    SelectModule,
    MultiSelectModule,
    DatePipe,
    DialogModule,
    LoadingComponent,
    TooltipComponent,
  ],
  templateUrl: './rotating-proxies.component.html',
  styleUrl: './rotating-proxies.component.scss'
})
export class RotatingProxiesComponent implements OnInit, OnDestroy {
  private readonly protocolOptionList: { label: string; value: string }[] = [
    {label: 'HTTP', value: 'http'},
    {label: 'HTTPS', value: 'https'},
    {label: 'SOCKS4', value: 'socks4'},
    {label: 'SOCKS5', value: 'socks5'},
  ];
  private readonly transportProtocolOptionList: { label: string; value: string }[] = [
    {label: 'TCP', value: 'tcp'},
    {label: 'QUIC', value: 'quic'},
    {label: 'HTTP/3', value: 'http3'},
  ];
  readonly transportProtocolTooltip =
    'TCP uses standard HTTP over TCP. QUIC and HTTP/3 both use HTTP/3 over QUIC; QUIC enables HTTP/3 datagrams (unreliable messages), HTTP/3 uses streams only.';
  createForm: FormGroup;
  rotatingProxies = signal<RotatingProxy[]>([]);
  protocolOptions = signal<{ label: string; value: string }[]>([]);
  listenProtocolOptions = signal<{ label: string; value: string }[]>([...this.protocolOptionList]);
  transportProtocolOptions = [...this.transportProtocolOptionList];
  loading = signal(false);
  submitting = signal(false);
  rotateLoading = signal<Set<number>>(new Set());
  noProtocolsAvailable = signal(false);
  authEnabled = signal(false);
  selectedRotator = signal<RotatingProxy | null>(null);
  detailsVisible = signal(false);
  readonly reputationOptions = [
    {label: 'Good', value: 'good'},
    {label: 'Neutral', value: 'neutral'},
    {label: 'Bad', value: 'poor'},
  ];
  private readonly allReputationValues = this.reputationOptions.map(option => option.value);
  private readonly reputationDisplay: Record<string, string> = {
    good: 'Good',
    neutral: 'Neutral',
    poor: 'Bad',
  };

  private readonly loopbackHost = '127.0.0.1';
  private readonly defaultRotatorHost = this.resolveDefaultHost();
  rotatorHost = signal(this.loopbackHost);
  private destroy$ = new Subject<void>();

  constructor(private fb: FormBuilder, private http: HttpService) {
    this.createForm = this.fb.group({
      name: ['', [Validators.required, Validators.maxLength(120)]],
      protocol: ['', Validators.required],
      listenProtocol: ['', Validators.required],
      transportProtocol: ['tcp', Validators.required],
      listenTransportProtocol: ['tcp', Validators.required],
      authRequired: [false],
      authUsername: [{value: '', disabled: true}, [Validators.maxLength(120)]],
      authPassword: [{value: '', disabled: true}, [Validators.maxLength(120)]],
      reputationLabels: [this.getDefaultReputationSelection()],
    });
  }

  ngOnInit(): void {
    this.createForm.get('authRequired')?.valueChanges
      .pipe(takeUntil(this.destroy$))
      .subscribe(value => {
        const enabled = !!value;
        this.authEnabled.set(enabled);
        this.updateAuthControls(enabled, this.submitting());
      });

    this.loadInitialData();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadInitialData(): void {
    this.loading.set(true);
    forkJoin({
      proxies: this.http.getRotatingProxies(),
      settings: this.http.getUserSettings(),
    })
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: ({proxies, settings}) => {
          const rawProxies = proxies ?? [];
          const currentSelectedId = this.selectedRotator()?.id ?? null;
          if (!this.rotatorHost()) {
            this.rotatorHost.set(this.loopbackHost || this.defaultRotatorHost);
          }

          const enriched = rawProxies.map(proxy => this.enrichRotator(proxy));
          this.rotatingProxies.set(enriched);
          if (currentSelectedId) {
            const current = enriched.find(item => item.id === currentSelectedId) ?? null;
            this.selectedRotator.set(current);
            if (!current && this.detailsVisible()) {
              this.detailsVisible.set(false);
            }
          } else if (this.selectedRotator()) {
            const updated = enriched.find(item => item.id === this.selectedRotator()?.id) ?? null;
            this.selectedRotator.set(updated);
            if (!updated && this.detailsVisible()) {
              this.detailsVisible.set(false);
            }
          }
          if (!this.selectedRotator() && this.detailsVisible()) {
            this.detailsVisible.set(false);
          }

          const options = this.buildProtocolOptions(settings);
          this.protocolOptions.set(options);
          const noneAvailable = options.length === 0;
          this.noProtocolsAvailable.set(noneAvailable);
          const protocolControl = this.createForm.get('protocol');
          const listenControl = this.createForm.get('listenProtocol');
          const transportControl = this.createForm.get('transportProtocol');
          const listenTransportControl = this.createForm.get('listenTransportProtocol');
          const availableValues = options.map(opt => opt.value);
          const currentProtocol = protocolControl?.value;
          const currentListen = listenControl?.value;
          if (!currentProtocol || !availableValues.includes(currentProtocol)) {
            this.createForm.patchValue({protocol: options[0]?.value ?? ''}, {emitEvent: false});
          }
          const listenOptions = this.listenProtocolOptions().map(opt => opt.value);
          if (!currentListen || !listenOptions.includes(currentListen)) {
            this.createForm.patchValue({listenProtocol: listenOptions[0] ?? ''}, {emitEvent: false});
          }
          const transportValues = this.transportProtocolOptions.map(opt => opt.value);
          const currentTransport = transportControl?.value;
          const currentListenTransport = listenTransportControl?.value;
          if (!currentTransport || !transportValues.includes(currentTransport)) {
            this.createForm.patchValue({transportProtocol: transportValues[0] ?? 'tcp'}, {emitEvent: false});
          }
          if (!currentListenTransport || !transportValues.includes(currentListenTransport)) {
            this.createForm.patchValue({listenTransportProtocol: transportValues[0] ?? 'tcp'}, {emitEvent: false});
          }
          this.updateFormDisabledStates();
          this.loading.set(false);
        },
        error: err => {
          this.loading.set(false);
          NotificationService.showError('Failed to load rotating proxies: ' + this.getErrorMessage(err));
        }
      });
  }

  createRotator(): void {
    if (this.createForm.invalid || this.submitting()) {
      this.createForm.markAllAsTouched();
      return;
    }

    const payload: CreateRotatingProxy = {
      name: (this.createForm.get('name')?.value ?? '').trim(),
      protocol: this.createForm.get('protocol')?.value,
      listen_protocol: this.createForm.get('listenProtocol')?.value,
      transport_protocol: this.createForm.get('transportProtocol')?.value,
      listen_transport_protocol: this.createForm.get('listenTransportProtocol')?.value,
      auth_required: !!this.createForm.get('authRequired')?.value,
    };

    const reputationSelection = this.normalizeReputationSelection(this.createForm.get('reputationLabels')?.value);
    if (reputationSelection.length > 0) {
      payload.reputation_labels = reputationSelection;
    }

    if (payload.auth_required) {
      payload.auth_username = (this.createForm.get('authUsername')?.value ?? '').trim();
      payload.auth_password = this.createForm.get('authPassword')?.value ?? '';
    }

    if (!payload.name) {
      this.createForm.get('name')?.setValue('');
      this.createForm.get('name')?.markAsTouched();
      NotificationService.showWarn('Name cannot be empty.');
      return;
    }

    this.submitting.set(true);
    this.updateFormDisabledStates();
    this.updateAuthControls(!!this.createForm.get('authRequired')?.value, true);
    this.http.createRotatingProxy(payload)
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: proxy => {
          const enriched = this.enrichRotator(proxy);
          if (enriched.listen_host) {
            this.rotatorHost.set(enriched.listen_host);
          } else if (!this.rotatorHost()) {
            this.rotatorHost.set(this.defaultRotatorHost);
          }
          this.rotatingProxies.update(list => [enriched, ...list]);
          if (this.detailsVisible()) {
            this.selectedRotator.set(enriched);
          }
          this.submitting.set(false);
          this.updateFormDisabledStates();
          this.updateAuthControls(!!this.createForm.get('authRequired')?.value, false);
          this.createForm.patchValue({name: ''}, {emitEvent: false});
          this.createForm.get('authUsername')?.reset('', {emitEvent: false});
          this.createForm.get('authPassword')?.reset('', {emitEvent: false});
          NotificationService.showSuccess('Rotating proxy created.');
        },
        error: err => {
          this.submitting.set(false);
          this.updateFormDisabledStates();
          this.updateAuthControls(!!this.createForm.get('authRequired')?.value, false);
          NotificationService.showError('Could not create rotating proxy: ' + this.getErrorMessage(err));
        }
      });
  }

  deleteProxy(proxy: RotatingProxy): void {
    if (!proxy) {
      return;
    }

    this.http.deleteRotatingProxy(proxy.id)
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: () => {
          this.rotatingProxies.update(list => list.filter(item => item.id !== proxy.id));
          const currentSelected = this.selectedRotator();
          if (currentSelected && currentSelected.id === proxy.id) {
            this.selectedRotator.set(null);
            this.detailsVisible.set(false);
          }
          NotificationService.showSuccess('Rotating proxy deleted.');
        },
        error: err => {
          NotificationService.showError('Could not delete rotating proxy: ' + this.getErrorMessage(err));
        }
      });
  }

  rotate(proxy: RotatingProxy): void {
    if (!proxy || this.isRotating(proxy.id)) {
      return;
    }

    this.rotateLoading.update(set => {
      const next = new Set(set);
      next.add(proxy.id);
      return next;
    });
    this.http.getNextRotatingProxy(proxy.id)
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: res => {
          this.rotateLoading.update(set => {
            const next = new Set(set);
            next.delete(proxy.id);
            return next;
          });
          const address = `${res.ip}:${res.port}`;
          let updatedRotator: RotatingProxy | null = null;
          this.rotatingProxies.update(list =>
            list.map(item => {
              if (item.id !== proxy.id) {
                return item;
              }
              const enriched = this.enrichRotator({
                ...item,
                last_served_proxy: address,
                last_rotation_at: new Date().toISOString(),
              });
              updatedRotator = enriched;
              return enriched;
            })
          );
          if (!updatedRotator) {
            updatedRotator = this.enrichRotator(proxy);
          }
          if (updatedRotator.listen_host) {
            this.rotatorHost.set(updatedRotator.listen_host);
          }
          const currentSelected = this.selectedRotator();
          if (currentSelected && currentSelected.id === updatedRotator.id) {
            this.selectedRotator.set(updatedRotator);
          }
          NotificationService.showSuccess(`Serving ${address}`);
        },
        error: err => {
          this.rotateLoading.update(set => {
            const next = new Set(set);
            next.delete(proxy.id);
            return next;
          });
          NotificationService.showError('Could not rotate proxy: ' + this.getErrorMessage(err));
        }
      });
  }

  copyRotatorConnection(proxy: RotatingProxy | null): void {
    if (!proxy) {
      NotificationService.showWarn('Rotator connection is not available yet.');
      return;
    }

    const connection = this.rotatorConnectionString(proxy);
    this.copyValueToClipboard(connection, 'Rotator connection copied.', 'Rotator connection is not available yet.');
  }

  copyRotatorField(value: string | null | undefined, label: string): void {
    this.copyValueToClipboard(value ?? '', `${label} copied.`, `${label} is not set.`);
  }

  showRotatorDetails(proxy: RotatingProxy): void {
    this.selectedRotator.set(proxy);
    this.detailsVisible.set(true);
  }

  onDetailsHide(): void {
    this.detailsVisible.set(false);
  }

  isRotating(id: number): boolean {
    return this.rotateLoading().has(id);
  }

  rotatorEndpoint(proxy: RotatingProxy | null | undefined): string {
    if (!proxy) {
      return '';
    }
    const address = (proxy.listen_address ?? '').toString().trim();
    if (address) {
      return address;
    }

    const host = (proxy.listen_host ?? '').toString().trim();
    if (host) {
      return `${host}:${proxy.listen_port}`;
    }

    return `${proxy.listen_port}`;
  }

  rotatorConnectionString(proxy: RotatingProxy | null | undefined): string {
    if (!proxy) {
      return '';
    }
    const endpoint = this.rotatorEndpoint(proxy);
    if (!endpoint) {
      return '';
    }

    const protocol = (proxy.listen_protocol ?? proxy.protocol ?? '').toLowerCase() || 'http';
    const needsAuth = proxy.auth_required && !!proxy.auth_username && !!proxy.auth_password;
    const credentials = needsAuth ? `${proxy.auth_username}:${proxy.auth_password}@` : '';
    return `${protocol}://${credentials}${endpoint}`;
  }

  protocolLabel(value: string): string {
    switch (value) {
      case 'http':
        return 'HTTP';
      case 'https':
        return 'HTTPS';
      case 'socks4':
        return 'SOCKS4';
      case 'socks5':
        return 'SOCKS5';
      default:
        return value?.toUpperCase() ?? '';
    }
  }

  transportLabel(value: string): string {
    switch ((value ?? '').toLowerCase()) {
      case 'tcp':
        return 'TCP';
      case 'quic':
        return 'QUIC';
      case 'http3':
        return 'HTTP/3';
      default:
        return value?.toUpperCase() ?? '';
    }
  }

  reputationFilterSummary(labels: string[] | null | undefined): string {
    const normalized = this.normalizeReputationSelection(labels);
    if (normalized.length === 0 || normalized.length === this.allReputationValues.length) {
      return 'All reputations';
    }
    return normalized
      .map(label => this.reputationDisplay[label] ?? label.toUpperCase())
      .join(', ');
  }

  private buildProtocolOptions(settings: UserSettings | null | undefined): { label: string; value: string }[] {
    if (!settings) {
      return [];
    }

    const options: { label: string; value: string }[] = [];
    if (settings.http_protocol) {
      options.push({label: 'HTTP', value: 'http'});
    }
    if (settings.https_protocol) {
      options.push({label: 'HTTPS', value: 'https'});
    }
    if (settings.socks4_protocol) {
      options.push({label: 'SOCKS4', value: 'socks4'});
    }
    if (settings.socks5_protocol) {
      options.push({label: 'SOCKS5', value: 'socks5'});
    }
    return options;
  }

  private updateFormDisabledStates(): void {
    const submitting = this.submitting();
    const noneAvailable = this.noProtocolsAvailable();

    this.setDisabledState('name', submitting || noneAvailable);
    this.setDisabledState('protocol', submitting || noneAvailable);
    this.setDisabledState('listenProtocol', submitting || noneAvailable);
    this.setDisabledState('transportProtocol', submitting || noneAvailable);
    this.setDisabledState('listenTransportProtocol', submitting || noneAvailable);
    this.setDisabledState('reputationLabels', submitting);
    this.setDisabledState('authRequired', submitting);
  }

  private updateAuthControls(requireAuth: boolean, submitting: boolean): void {
    const usernameControl = this.createForm.get('authUsername');
    const passwordControl = this.createForm.get('authPassword');
    const disableFields = submitting || !requireAuth;

    if (requireAuth) {
      usernameControl?.addValidators(Validators.required);
      passwordControl?.addValidators(Validators.required);
    } else {
      usernameControl?.removeValidators(Validators.required);
      passwordControl?.removeValidators(Validators.required);
    }

    if (disableFields) {
      if (!requireAuth) {
        usernameControl?.reset('', {emitEvent: false});
        passwordControl?.reset('', {emitEvent: false});
      }
      usernameControl?.disable({emitEvent: false});
      passwordControl?.disable({emitEvent: false});
    } else {
      usernameControl?.enable({emitEvent: false});
      passwordControl?.enable({emitEvent: false});
    }

    usernameControl?.updateValueAndValidity({emitEvent: false});
    passwordControl?.updateValueAndValidity({emitEvent: false});
  }

  private setDisabledState(controlName: string, shouldDisable: boolean): void {
    const control = this.createForm.get(controlName);
    if (!control) {
      return;
    }
    if (shouldDisable) {
      control.disable({emitEvent: false});
    } else {
      control.enable({emitEvent: false});
    }
  }

  private getErrorMessage(err: any): string {
    return err?.error?.error ?? err?.error?.message ?? err?.message ?? 'Unknown error';
  }

  private enrichRotator(proxy: RotatingProxy): RotatingProxy {
    const listenHost = this.resolveHostValue(proxy.listen_host);
    const listenAddress = listenHost ? `${listenHost}:${proxy.listen_port}` : `${proxy.listen_port}`;
    const transportProtocol = (proxy.transport_protocol ?? 'tcp').toString().trim().toLowerCase() || 'tcp';
    const listenTransportProtocol = (proxy.listen_transport_protocol ?? transportProtocol).toString().trim().toLowerCase() || transportProtocol;

    return {
      ...proxy,
      listen_protocol: proxy.listen_protocol ?? proxy.protocol ?? null,
      transport_protocol: transportProtocol,
      listen_transport_protocol: listenTransportProtocol,
      auth_username: proxy.auth_username ?? null,
      auth_password: proxy.auth_password ?? null,
      listen_host: listenHost || null,
      listen_address: listenAddress,
      reputation_labels: this.normalizeReputationSelection(proxy.reputation_labels),
    };
  }

  private getDefaultReputationSelection(): string[] {
    return [...this.allReputationValues];
  }

  private normalizeReputationSelection(raw: string[] | null | undefined): string[] {
    if (!raw || raw.length === 0) {
      return [];
    }
    const normalized: string[] = [];
    for (const allowed of this.allReputationValues) {
      if (raw.includes(allowed)) {
        normalized.push(allowed);
      }
    }
    return normalized;
  }

  private resolveHostValue(host: string | null | undefined): string {
    if (this.loopbackHost) {
      return this.loopbackHost;
    }
    const candidate = (host ?? '').toString().trim();
    if (candidate) {
      return candidate;
    }
    if (this.rotatorHost()) {
      return this.rotatorHost();
    }
    if (this.defaultRotatorHost) {
      return this.defaultRotatorHost;
    }
    if (typeof window !== 'undefined' && window.location?.hostname) {
      return window.location.hostname;
    }
    return '';
  }

  private copyValueToClipboard(value: string, successMessage: string, emptyMessage: string): void {
    if (!value) {
      NotificationService.showWarn(emptyMessage);
      return;
    }

    if (navigator?.clipboard?.writeText) {
      navigator.clipboard.writeText(value)
        .then(() => NotificationService.showSuccess(successMessage))
        .catch(() => NotificationService.showWarn('Could not copy to clipboard.'));
    } else {
      NotificationService.showWarn('Clipboard access is not available.');
    }
  }

  private resolveDefaultHost(): string {
    try {
      const url = new URL(environment.apiUrl);
      if (url.hostname) {
        return url.hostname;
      }
    } catch (err) {
      // Ignore parse errors and fall back below
    }

    if (typeof window !== 'undefined' && window.location?.hostname) {
      return window.location.hostname;
    }
    return '';
  }
}
