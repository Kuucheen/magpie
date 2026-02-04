import {Component, OnDestroy, OnInit} from '@angular/core';
import {FormArray, FormBuilder, FormControl, FormGroup, ReactiveFormsModule} from "@angular/forms";
import {SettingsService} from '../../services/settings.service';
import {Subject} from 'rxjs';
import {filter, take, takeUntil} from 'rxjs/operators';

import {TabsModule} from 'primeng/tabs';
import {SelectModule} from 'primeng/select';
import {InputNumberModule} from 'primeng/inputnumber';
import {ButtonModule} from 'primeng/button';
import {DividerModule} from 'primeng/divider';
import {TooltipModule} from 'primeng/tooltip';
import {CheckboxModule} from 'primeng/checkbox';
import {InputTextModule} from 'primeng/inputtext';
import {NotificationService} from '../../services/notification-service.service';
import {GlobalSettings} from '../../models/GlobalSettings';

@Component({
  selector: 'app-admin-scraper',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    TabsModule,
    SelectModule,
    InputNumberModule,
    ButtonModule,
    DividerModule,
    TooltipModule,
    CheckboxModule,
    InputTextModule
  ],
  templateUrl: './admin-scraper.component.html',
  styleUrl: './admin-scraper.component.scss'
})
export class AdminScraperComponent implements OnInit, OnDestroy {
  daysList = Array.from({ length: 31 }, (_, i) => ({ label: `${i} Days`, value: i }));
  hoursList = Array.from({ length: 24 }, (_, i) => ({ label: `${i} Hours`, value: i }));
  minutesList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Minutes`, value: i }));
  secondsList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Seconds`, value: i }));
  settingsForm: FormGroup;
  private destroy$ = new Subject<void>();

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private notification: NotificationService
  ) {
    this.settingsForm = this.createDefaultForm();
  }

  ngOnInit(): void {
    this.settingsService.settings$
      .pipe(
        filter((settings): settings is GlobalSettings => !!settings),
        take(1)
      )
      .subscribe({
        next: settings => this.updateFormWithSettings(settings),
        error: err => this.notification.showError("Could not get scraper settings" + err.error.message)
    });

    const threadsCtrl  = this.settingsForm.get('scraper_threads');
    const dynamicCtrl  = this.settingsForm.get('scraper_dynamic_threads');
    const proxyLimitCtrl = this.settingsForm.get('proxy_limit_enabled');

    /* whenever the checkbox toggles, enable/disable "threads" */
    dynamicCtrl!.valueChanges
      .pipe(takeUntil(this.destroy$))
      .subscribe({
      next: (isDynamic: boolean) => {
        this.updateThreadControlState(isDynamic);
      }, error: err => this.notification.showError("Error while toggling threadCtrl: " + err.error.message)
    });

    proxyLimitCtrl!.valueChanges
      .pipe(takeUntil(this.destroy$))
      .subscribe({
      next: (enabled: boolean) => {
        this.updateProxyLimitState(enabled);
      }, error: err => this.notification.showError("Error while toggling proxy limit: " + err.error.message)
    });

    this.updateThreadControlState(dynamicCtrl?.value ?? true);
    this.updateProxyLimitState(proxyLimitCtrl?.value ?? false);
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  private createDefaultForm(): FormGroup {
    return this.fb.group({
      scraper_dynamic_threads: true,
      scraper_threads: [{ value: 250, disabled: true }],
      scraper_retries: [2],
      scraper_timeout: [7500],
      scraper_respect_robots_txt: [true],
      scraper_timer: this.fb.group({
        days: [0],
        hours: [9],
        minutes: [0],
        seconds: [0]
      }),
      scrape_sites: this.fb.array([this.createScrapeSiteControl()]),
      proxy_limit_enabled: [false],
      proxy_limit_max_per_user: [0],
      proxy_limit_exclude_admins: [true]
    });
  }

  private updateFormWithSettings(settings: GlobalSettings): void {
    this.settingsForm.patchValue({
      scraper_dynamic_threads: settings.scraper.dynamic_threads,
      scraper_threads: settings.scraper.threads,
      scraper_retries: settings.scraper.retries,
      scraper_timeout: settings.scraper.timeout,
      scraper_respect_robots_txt: settings.scraper.respect_robots_txt ?? true,
      scraper_timer: {
        days: settings.scraper.scraper_timer.days,
        hours: settings.scraper.scraper_timer.hours,
        minutes: settings.scraper.scraper_timer.minutes,
        seconds: settings.scraper.scraper_timer.seconds
      },
      proxy_limit_enabled: settings.proxy_limits.enabled,
      proxy_limit_max_per_user: settings.proxy_limits.max_per_user,
      proxy_limit_exclude_admins: settings.proxy_limits.exclude_admins
    });

    this.resetScrapeSites(settings.scraper.scrape_sites);
    this.updateThreadControlState(settings.scraper.dynamic_threads);
    this.updateProxyLimitState(settings.proxy_limits.enabled);
  }

  private updateThreadControlState(isDynamic: boolean): void {
    const threadsCtrl  = this.settingsForm.get('scraper_threads');
    if (!threadsCtrl) {
      return;
    }

    if (isDynamic) {
      threadsCtrl.disable({ emitEvent: false });
    } else {
      threadsCtrl.enable({ emitEvent: false });
    }
  }

  private updateProxyLimitState(isEnabled: boolean): void {
    const maxCtrl = this.settingsForm.get('proxy_limit_max_per_user');
    const excludeCtrl = this.settingsForm.get('proxy_limit_exclude_admins');

    if (!maxCtrl || !excludeCtrl) {
      return;
    }

    if (isEnabled) {
      maxCtrl.enable({ emitEvent: false });
      excludeCtrl.enable({ emitEvent: false });
    } else {
      maxCtrl.disable({ emitEvent: false });
      excludeCtrl.disable({ emitEvent: false });
    }
  }

  get scrapeSites(): FormArray<FormControl<string>> {
    return this.settingsForm.get('scrape_sites') as FormArray<FormControl<string>>;
  }

  addScrapeSite(): void {
    this.scrapeSites.push(this.createScrapeSiteControl());
    this.settingsForm.markAsDirty();
  }

  removeScrapeSite(index: number): void {
    if (index < 0 || index >= this.scrapeSites.length) {
      return;
    }

    if (this.scrapeSites.length === 1) {
      this.scrapeSites.at(0).setValue('');
    } else {
      this.scrapeSites.removeAt(index);
    }
    this.settingsForm.markAsDirty();
  }

  private resetScrapeSites(sites: string[]): void {
    this.scrapeSites.clear();

    if (!sites || sites.length === 0) {
      this.scrapeSites.push(this.createScrapeSiteControl());
    } else {
      sites.forEach(site => this.scrapeSites.push(this.createScrapeSiteControl(site)));
    }

    this.scrapeSites.markAsPristine();
  }

  private createScrapeSiteControl(value: string = ''): FormControl<string> {
    return this.fb.nonNullable.control(value);
  }

  onSubmit() {
    this.settingsService.saveGlobalSettings(this.settingsForm.getRawValue()).subscribe({
      next: (resp) => {
        this.notification.showSuccess(resp.message)
        this.settingsForm.markAsPristine()
      },
      error: (err) => {
        console.error("Error saving settings:", err);
        const reason = err?.error?.message ?? err?.error?.error ?? 'Unknown error';
        this.notification.showError("Failed to save settings: " + reason);
      }
    });
  }
}
