import {Component, OnDestroy, OnInit, signal} from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import {FormBuilder, FormGroup, ReactiveFormsModule, Validators} from '@angular/forms';
import {forkJoin, Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';
import {TableModule} from 'primeng/table';
import {ButtonModule} from 'primeng/button';
import {InputTextModule} from 'primeng/inputtext';
import {SelectModule} from 'primeng/select';
import {DialogModule} from 'primeng/dialog';
import {MultiSelectModule} from 'primeng/multiselect';
import {Chip} from 'primeng/chip';
import {TooltipModule} from 'primeng/tooltip';

import {environment} from '../../environments/environment';

import {HttpService} from '../services/http.service';
import {ClipboardService} from '../services/clipboard.service';
import {NotificationService} from '../services/notification-service.service';
import {CreateRotatingProxy, RotatingProxy, RotatingProxyInstance} from '../models/RotatingProxy';
import {UserSettings} from '../models/UserSettings';
import {TooltipComponent} from '../tooltip/tooltip.component';
import {SkeletonModule} from 'primeng/skeleton';

type RotatorInstanceOption = {
  label: string;
  value: string;
  freePorts: number;
};

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
    Chip,
    DatePipe,
    DialogModule,
    TooltipComponent,
    TooltipModule,
    SkeletonModule,
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
  instanceOptions = signal<RotatorInstanceOption[]>([]);
  hasAvailableInstances = signal(false);
  loading = signal(false);
  hasLoaded = signal(false);
  submitting = signal(false);
  rotateLoading = signal<Set<number>>(new Set());
  noProtocolsAvailable = signal(false);
  authEnabled = signal(false);
  selectedRotator = signal<RotatingProxy | null>(null);
  detailsVisible = signal(false);
  copiedDetailField = signal<'endpoint' | 'connection' | 'ip' | 'port' | 'username' | 'password' | null>(null);
  readonly reputationOptions = [
    {label: 'Good', value: 'good'},
    {label: 'Neutral', value: 'neutral'},
    {label: 'Bad', value: 'poor'},
  ];
  readonly uptimeFilterTypeOptions = [
    {label: 'Minimum uptime', value: 'min'},
    {label: 'Maximum uptime', value: 'max'},
  ];
  private readonly allReputationValues = this.reputationOptions.map(option => option.value);
  private readonly reputationDisplay: Record<string, string> = {
    good: 'Good',
    neutral: 'Neutral',
    poor: 'Bad',
  };
  readonly skeletonRows = Array.from({ length: 5 });

  private readonly loopbackHost = '127.0.0.1';
  private readonly defaultRotatorHost = this.resolveDefaultHost();
  rotatorHost = signal(this.loopbackHost);
  private destroy$ = new Subject<void>();
  private authCopyFeedbackTimeout?: ReturnType<typeof setTimeout>;

  constructor(
    private fb: FormBuilder,
    private http: HttpService,
    private clipboardService: ClipboardService,
    private notification: NotificationService
  ) {
    this.createForm = this.fb.group({
      name: ['', [Validators.required, Validators.maxLength(120)]],
      instanceId: ['', Validators.required],
      protocol: ['', Validators.required],
      listenProtocol: ['', Validators.required],
      transportProtocol: ['tcp', Validators.required],
      listenTransportProtocol: ['tcp', Validators.required],
      uptimeFilterType: ['min'],
      uptimePercentage: [null, [Validators.min(0), Validators.max(100)]],
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
    if (this.authCopyFeedbackTimeout) {
      clearTimeout(this.authCopyFeedbackTimeout);
      this.authCopyFeedbackTimeout = undefined;
    }
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadInitialData(): void {
    this.loading.set(true);
    forkJoin({
      proxies: this.http.getRotatingProxies(),
      settings: this.http.getUserSettings(),
      instances: this.http.getRotatingProxyInstances(),
    })
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: ({proxies, settings, instances}) => {
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
          this.applyInstanceOptions(instances ?? []);
          const noneAvailable = options.length === 0;
          this.noProtocolsAvailable.set(noneAvailable);
          const instanceControl = this.createForm.get('instanceId');
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
          const availableInstances = this.instanceOptions().map(option => option.value);
          const currentInstance = instanceControl?.value;
          if (!currentInstance || !availableInstances.includes(currentInstance)) {
            this.createForm.patchValue({instanceId: availableInstances[0] ?? ''}, {emitEvent: false});
          }
          this.updateFormDisabledStates();
          this.loading.set(false);
          this.hasLoaded.set(true);
      },
      error: err => {
        this.loading.set(false);
        this.hasLoaded.set(true);
        this.notification.showError('Failed to load rotating proxies: ' + this.getErrorMessage(err));
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
      instance_id: this.createForm.get('instanceId')?.value,
      protocol: this.createForm.get('protocol')?.value,
      listen_protocol: this.createForm.get('listenProtocol')?.value,
      transport_protocol: this.createForm.get('transportProtocol')?.value,
      listen_transport_protocol: this.createForm.get('listenTransportProtocol')?.value,
      auth_required: !!this.createForm.get('authRequired')?.value,
    };

    const uptimePercentage = this.normalizeUptimePercentage(this.createForm.get('uptimePercentage')?.value);
    if (uptimePercentage !== null) {
      payload.uptime_filter_type = this.normalizeUptimeFilterType(this.createForm.get('uptimeFilterType')?.value) || 'min';
      payload.uptime_percentage = uptimePercentage;
    }

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
      this.notification.showWarn('Name cannot be empty.');
      return;
    }
    if (!payload.instance_id) {
      this.createForm.get('instanceId')?.setValue('');
      this.createForm.get('instanceId')?.markAsTouched();
      this.notification.showWarn('Please select an instance.');
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
          this.loadInitialData();
          this.notification.showSuccess('Rotating proxy created.');
        },
        error: err => {
          this.submitting.set(false);
          this.updateFormDisabledStates();
          this.updateAuthControls(!!this.createForm.get('authRequired')?.value, false);
          this.notification.showError('Could not create rotating proxy: ' + this.getErrorMessage(err));
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
          this.notification.showSuccess('Rotating proxy deleted.');
        },
        error: err => {
          this.notification.showError('Could not delete rotating proxy: ' + this.getErrorMessage(err));
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
          this.notification.showSuccess(`Serving ${address}`);
        },
        error: err => {
          this.rotateLoading.update(set => {
            const next = new Set(set);
            next.delete(proxy.id);
            return next;
          });
          this.notification.showError('Could not rotate proxy: ' + this.getErrorMessage(err));
        }
      });
  }

  isDetailFieldCopied(field: 'endpoint' | 'connection' | 'ip' | 'port' | 'username' | 'password'): boolean {
    return this.copiedDetailField() === field;
  }

  copyDetailField(field: 'endpoint' | 'connection' | 'ip' | 'port' | 'username' | 'password', value: string | null | undefined): void {
    const candidate = (value ?? '').toString();
    if (!candidate) {
      return;
    }

    this.showCopyFeedback(field);
    this.clipboardService.copyText(candidate).then(copied => {
      if (!copied) {
        this.copiedDetailField.set(null);
        this.notification.showWarn('Could not copy to clipboard.');
      }
    });
  }

  showRotatorDetails(proxy: RotatingProxy): void {
    this.selectedRotator.set(proxy);
    this.detailsVisible.set(true);
  }

  onDetailsHide(): void {
    if (this.authCopyFeedbackTimeout) {
      clearTimeout(this.authCopyFeedbackTimeout);
      this.authCopyFeedbackTimeout = undefined;
    }
    this.detailsVisible.set(false);
    this.copiedDetailField.set(null);
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

  uptimeFilterSummary(type: string | null | undefined, percentage: number | null | undefined): string {
    const normalizedType = this.normalizeUptimeFilterType(type);
    const normalizedPercentage = this.normalizeUptimePercentage(percentage);
    if (!normalizedType || normalizedPercentage === null) {
      return 'No uptime filter';
    }
    const comparator = normalizedType === 'max' ? '<=' : '>=';
    return `${comparator} ${normalizedPercentage}%`;
  }

  instanceSummary(instanceId: string | null | undefined): string {
    const id = (instanceId ?? '').toString().trim();
    if (!id) {
      return 'Not selected';
    }
    const option = this.instanceOptions().find(item => item.value === id);
    if (!option) {
      return id;
    }
    return `${option.label} (${option.freePorts} free)`;
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
    const noInstances = !this.hasAvailableInstances();

    this.setDisabledState('name', submitting || noneAvailable);
    this.setDisabledState('instanceId', submitting || noInstances);
    this.setDisabledState('protocol', submitting || noneAvailable);
    this.setDisabledState('listenProtocol', submitting || noneAvailable);
    this.setDisabledState('transportProtocol', submitting || noneAvailable);
    this.setDisabledState('listenTransportProtocol', submitting || noneAvailable);
    this.setDisabledState('uptimeFilterType', submitting);
    this.setDisabledState('uptimePercentage', submitting);
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
    const uptimeFilterType = this.normalizeUptimeFilterType(proxy.uptime_filter_type);
    const uptimePercentage = this.normalizeUptimePercentage(proxy.uptime_percentage);

    return {
      ...proxy,
      listen_protocol: proxy.listen_protocol ?? proxy.protocol ?? null,
      transport_protocol: transportProtocol,
      listen_transport_protocol: listenTransportProtocol,
      auth_username: proxy.auth_username ?? null,
      auth_password: proxy.auth_password ?? null,
      listen_host: listenHost || null,
      listen_address: listenAddress,
      uptime_filter_type: uptimeFilterType || null,
      uptime_percentage: uptimePercentage,
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

  private normalizeUptimeFilterType(raw: string | null | undefined): 'min' | 'max' | '' {
    const normalized = (raw ?? '').toString().trim().toLowerCase();
    if (normalized === 'min' || normalized === 'max') {
      return normalized;
    }
    return '';
  }

  private normalizeUptimePercentage(raw: number | string | null | undefined): number | null {
    if (raw === null || raw === undefined || raw === '') {
      return null;
    }
    const parsed = typeof raw === 'number' ? raw : Number(raw);
    if (!Number.isFinite(parsed) || parsed < 0 || parsed > 100) {
      return null;
    }
    return Math.round(parsed * 10) / 10;
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

  private showCopyFeedback(field: 'endpoint' | 'connection' | 'ip' | 'port' | 'username' | 'password'): void {
    this.copiedDetailField.set(field);
    if (this.authCopyFeedbackTimeout) {
      clearTimeout(this.authCopyFeedbackTimeout);
    }
    this.authCopyFeedbackTimeout = setTimeout(() => {
      if (this.copiedDetailField() === field) {
        this.copiedDetailField.set(null);
      }
    }, 1400);
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

  private applyInstanceOptions(instances: RotatingProxyInstance[] | null | undefined): void {
    const raw = instances ?? [];
    const options = raw
      .filter(instance => instance && instance.free_ports > 0)
      .map(instance => ({
        value: instance.id,
        freePorts: instance.free_ports,
        label: `${instance.name} · ${instance.region}`,
      }));

    this.instanceOptions.set(options);
    this.hasAvailableInstances.set(options.length > 0);
  }
}
