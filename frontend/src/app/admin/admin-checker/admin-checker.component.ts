import {Component, OnDestroy, OnInit} from '@angular/core';
import {CheckboxComponent} from "../../checkbox/checkbox.component";
import {FormArray, FormBuilder, FormGroup, FormsModule, ReactiveFormsModule} from "@angular/forms";

import {TooltipComponent} from "../../tooltip/tooltip.component";
import {SettingsService} from '../../services/settings.service';
import {take, takeUntil} from 'rxjs/operators';
import {Button} from 'primeng/button';
import {Tab, TabList, TabPanel, TabPanels, Tabs} from 'primeng/tabs';
import {Select} from 'primeng/select';
import {InputText} from 'primeng/inputtext';
import {NotificationService} from '../../services/notification-service.service';
import {Subject} from 'rxjs';

@Component({
  selector: 'app-admin-checker',
  standalone: true,
  imports: [
    CheckboxComponent,
    FormsModule,
    ReactiveFormsModule,
    TooltipComponent,
    Button,
    TabPanel,
    Select,
    Tabs,
    InputText,
    TabList,
    Tab,
    TabPanels,
  ],
    templateUrl: './admin-checker.component.html',
    styleUrl: './admin-checker.component.scss'
})
export class AdminCheckerComponent implements OnInit, OnDestroy {
  settingsForm: FormGroup;
  daysList = Array.from({ length: 31 }, (_, i) => ({ label: `${i} Days`, value: i }));
  hoursList = Array.from({ length: 24 }, (_, i) => ({ label: `${i} Hours`, value: i }));
  minutesList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Minutes`, value: i }));
  secondsList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Seconds`, value: i }));
  private destroy$ = new Subject<void>();

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private notification: NotificationService
  ) {
    this.settingsForm = this.createDefaultForm();
  }

  ngOnInit(): void {
    this.settingsService.getCheckerSettings().pipe(take(1)).subscribe({
      next: checkerSettings => {
        if (checkerSettings) {
          this.updateFormWithCheckerSettings(checkerSettings);
        }
      },
      error: err => {this.notification.showError("Could not get checker settings: " + err.error.message)}
    });

    const settings = this.settingsService.getGlobalSettings();
    if (settings) {
      this.updateProtocolsAndBlacklist(settings.protocols, settings.blacklist_sources);
    }

    this.settingsService.settings$
      .pipe(takeUntil(this.destroy$))
      .subscribe(settingsState => {
        if (!settingsState) {
          return;
        }
        this.updateProtocolsAndBlacklist(settingsState.protocols, settingsState.blacklist_sources);
      });

    const dynamicControl = this.settingsForm.get('dynamic_threads')!;
    const threadsControl = this.settingsForm.get('threads');

    // Check initial state and set the threads control accordingly.
    if (dynamicControl?.value) {
      threadsControl?.disable();
    } else {
      threadsControl?.enable();
    }

    dynamicControl?.valueChanges?.subscribe({
      next: dynamic => dynamic ? threadsControl?.disable() : threadsControl?.enable(),
      error: err => this.notification.showError("Could not get dynamic thread info " + err.error.message)
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }


  private createDefaultForm(): FormGroup {
    return this.fb.group({
      dynamic_threads: false,
      threads: [250],
      retries: [2],
      timeout: [7500],
      protocols: this.fb.group({
        http: [false],
        https: [true],
        socks4: [false],
        socks5: [false],
      }),
      checker_timer: this.fb.group({
        days: [0],
        hours: [1],
        minutes: [0],
        seconds: [0]
      }),
      judges_threads: [3],
      judges_timeout: [5000],
      judge_timer: this.fb.group({
        days: [0],
        hours: [0],
        minutes: [30],
        seconds: [0]
      }),
      judges: this.fb.array([
        this.fb.group({ url: ['https://pool.proxyspace.pro/judge.php'], regex: ['default'] }),
        this.fb.group({ url: ['http://azenv.net'], regex: ['default'] })
      ]),
      use_https_for_socks: true,
      iplookup: ['https://ident.me'],
      standard_header: this.fb.array([
        "USER-AGENT", "HOST", "ACCEPT", "ACCEPT-ENCODING"
      ]),
      proxy_header: this.fb.array([
        "HTTP_X_FORWARDED_FOR", "HTTP_FORWARDED", "HTTP_VIA", "HTTP_X_PROXY_ID"
      ]),
      blacklisted: this.fb.array([])
    });
  }

  private updateFormWithCheckerSettings(checkerSettings: any): void {
    // Update checker-specific fields
    this.settingsForm.patchValue({
      dynamic_threads: checkerSettings.dynamic_threads,
      threads: checkerSettings.threads,
      retries: checkerSettings.retries,
      timeout: checkerSettings.timeout,
      checker_timer: {
        days: checkerSettings.checker_timer.days,
        hours: checkerSettings.checker_timer.hours,
        minutes: checkerSettings.checker_timer.minutes,
        seconds: checkerSettings.checker_timer.seconds
      },
      iplookup: checkerSettings.ip_lookup,
      judges_threads: checkerSettings.judges_threads,
      judges_timeout: checkerSettings.judges_timeout,
      use_https_for_socks: checkerSettings.use_https_for_socks
    });

    // Update judge timer if exists
    if (checkerSettings.judge_timer) {
      this.settingsForm.patchValue({
        judge_timer: {
          days: checkerSettings.judge_timer.days,
          hours: checkerSettings.judge_timer.hours,
          minutes: checkerSettings.judge_timer.minutes,
          seconds: checkerSettings.judge_timer.seconds
        }
      });
    }

    // Update judges array
    this.updateJudgesArray(checkerSettings.judges);

    // Update headers arrays
    this.updateHeadersArrays(
      checkerSettings.standard_header || ["USER-AGENT", "HOST", "ACCEPT", "ACCEPT-ENCODING"],
      checkerSettings.proxy_header || ["HTTP_X_FORWARDED_FOR", "HTTP_FORWARDED", "HTTP_VIA", "HTTP_X_PROXY_ID"]
    );
  }

  private updateProtocolsAndBlacklist(protocols: any, blacklist: string[]): void {
    if (protocols) {
      // Check if protocols is an object with boolean values
      if (protocols && typeof protocols === 'object') {
        const protocolsGroup = this.settingsForm.get('protocols') as FormGroup;

        // Set checkboxes based on protocol values
        Object.keys(protocols).forEach(protocol => {
          if (protocolsGroup.contains(protocol)) {
            protocolsGroup.get(protocol)?.setValue(!!protocols[protocol]);
          }
        });
      }
    }

    // Update blacklist
    const blacklistArray = this.settingsForm.get('blacklisted') as FormArray;
    blacklistArray.clear();

    (blacklist ?? []).forEach(url => {
      blacklistArray.push(this.fb.control(url));
    });

    blacklistArray.markAsPristine();
  }

  private updateJudgesArray(judges: any[]): void {
    if (!judges || judges.length === 0) return;

    const judgesArray = this.settingsForm.get('judges') as FormArray;
    judgesArray.clear();

    judges.forEach(judge => {
      judgesArray.push(this.fb.group({
        url: [judge.url],
        regex: [judge.regex]
      }));
    });
  }

  private updateHeadersArrays(standardHeaders: string[], proxyHeaders: string[]): void {
    const standardHeaderArray = this.settingsForm.get('standard_header') as FormArray;
    standardHeaderArray.clear();
    standardHeaders.forEach(header => {
      standardHeaderArray.push(this.fb.control(header));
    });

    const proxyHeaderArray = this.settingsForm.get('proxy_header') as FormArray;
    proxyHeaderArray.clear();
    proxyHeaders.forEach(header => {
      proxyHeaderArray.push(this.fb.control(header));
    });
  }

  get judges() {
    return this.settingsForm.get('judges') as FormArray;
  }

  get blacklisted() {
    return this.settingsForm.get('blacklisted') as FormArray;
  }

  get standardHeaders() {
    return this.settingsForm.get('standard_header') as FormArray;
  }

  get proxyHeaders() {
    return this.settingsForm.get('proxy_header') as FormArray;
  }

  onSubmit() {
    this.settingsService.saveGlobalSettings(this.settingsForm.value).subscribe({
      next: (resp) => {
        this.notification.showSuccess(resp.message)
        this.settingsForm.markAsPristine()
      },
      error: (err) => {
        console.error("Error saving settings:", err);
        const reason = err?.error?.message ?? err?.error?.error ?? "Failed to save settings!";
        this.notification.showError(reason);
      }
    });
  }

  addJudge(): void {
    this.judges.push(this.fb.group({
      url: [''],
      regex: ['default']
    }));
    this.settingsForm.markAsDirty();
  }

  removeJudge(index: number): void {
    this.judges.removeAt(index);
    this.settingsForm.markAsDirty();
  }

  addBlacklistedUrl(): void {
    this.blacklisted.push(this.fb.control(''));
    this.settingsForm.markAsDirty();
  }

  removeBlacklistedUrl(index: number): void {
    this.blacklisted.removeAt(index);
    this.settingsForm.markAsDirty();
  }

  addStandardHeader(): void {
    this.standardHeaders.push(this.fb.control(''));
    this.settingsForm.markAsDirty();
  }

  removeStandardHeader(index: number): void {
    this.standardHeaders.removeAt(index);
    this.settingsForm.markAsDirty();
  }

  addProxyHeader(): void {
    this.proxyHeaders.push(this.fb.control(''));
    this.settingsForm.markAsDirty();
  }

  removeProxyHeader(index: number): void {
    this.proxyHeaders.removeAt(index);
    this.settingsForm.markAsDirty();
  }
}
