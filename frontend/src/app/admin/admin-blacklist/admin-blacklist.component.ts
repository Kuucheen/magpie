import {Component, OnDestroy, OnInit} from '@angular/core';
import {FormArray, FormBuilder, FormControl, FormGroup, ReactiveFormsModule} from '@angular/forms';
import {SettingsService} from '../../services/settings.service';
import {GlobalSettings} from '../../models/GlobalSettings';
import {Subject} from 'rxjs';
import {filter, takeUntil} from 'rxjs/operators';

import {SelectModule} from 'primeng/select';
import {InputTextModule} from 'primeng/inputtext';
import {ButtonModule} from 'primeng/button';
import {NotificationService} from '../../services/notification-service.service';

@Component({
  selector: 'app-admin-blacklist',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    InputTextModule,
    ButtonModule,
    SelectModule
  ],
  templateUrl: './admin-blacklist.component.html',
  styleUrl: './admin-blacklist.component.scss'
})
export class AdminBlacklistComponent implements OnInit, OnDestroy {
  daysList = Array.from({ length: 31 }, (_, i) => ({ label: `${i} Days`, value: i }));
  hoursList = Array.from({ length: 24 }, (_, i) => ({ label: `${i} Hours`, value: i }));
  minutesList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Minutes`, value: i }));
  secondsList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Seconds`, value: i }));

  form: FormGroup;
  private destroy$ = new Subject<void>();

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private notification: NotificationService
  ) {
    this.form = this.fb.group({
      blacklist_timer: this.fb.group({
        days: [0],
        hours: [6],
        minutes: [0],
        seconds: [0]
      }),
      blacklist_sources: this.fb.array([this.createSourceControl()]),
      website_blacklist: this.fb.array([this.createWebsiteControl()])
    });
  }

  ngOnInit(): void {
    this.settingsService.settings$
      .pipe(
        filter((settings): settings is GlobalSettings => !!settings),
        takeUntil(this.destroy$)
      )
      .subscribe(settings => this.applySettings(settings));
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  get sources(): FormArray<FormControl<string>> {
    return this.form.get('blacklist_sources') as FormArray<FormControl<string>>;
  }

  get blockedSites(): FormArray<FormControl<string>> {
    return this.form.get('website_blacklist') as FormArray<FormControl<string>>;
  }

  addSource(): void {
    this.sources.push(this.createSourceControl());
    this.form.markAsDirty();
  }

  removeSource(index: number): void {
    if (index < 0 || index >= this.sources.length) {
      return;
    }

    if (this.sources.length === 1) {
      this.sources.at(0).setValue('');
    } else {
      this.sources.removeAt(index);
    }
    this.form.markAsDirty();
  }

  addBlockedSite(): void {
    this.blockedSites.push(this.createWebsiteControl());
    this.form.markAsDirty();
  }

  removeBlockedSite(index: number): void {
    if (index < 0 || index >= this.blockedSites.length) {
      return;
    }

    if (this.blockedSites.length === 1) {
      this.blockedSites.at(0).setValue('');
    } else {
      this.blockedSites.removeAt(index);
    }
    this.form.markAsDirty();
  }

  onSubmit(): void {
    this.settingsService.saveGlobalSettings(this.form.getRawValue()).subscribe({
      next: (resp) => {
        this.notification.showSuccess(resp.message ?? 'Settings saved');
        this.form.markAsPristine();
      },
      error: (err) => {
        console.error('Error saving blacklist settings:', err);
        const reason = err?.error?.message ?? err?.error?.error ?? 'Unknown error';
        this.notification.showError('Failed to save blacklist settings: ' + reason);
      }
    });
  }

  private createSourceControl(value: string = ''): FormControl<string> {
    return this.fb.nonNullable.control(value);
  }

  private createWebsiteControl(value: string = ''): FormControl<string> {
    return this.fb.nonNullable.control(value);
  }

  private applySettings(settings: GlobalSettings): void {
    const timer = settings.blacklist_timer ?? { days: 0, hours: 6, minutes: 0, seconds: 0 };
    this.form.patchValue({
      blacklist_timer: {
        days: timer.days ?? 0,
        hours: timer.hours ?? 6,
        minutes: timer.minutes ?? 0,
        seconds: timer.seconds ?? 0
      }
    }, { emitEvent: false });

    this.resetSources(settings.blacklist_sources);
    this.resetWebsiteBlacklist(settings.website_blacklist);
    this.form.markAsPristine();
  }

  private resetSources(sources: string[] = []): void {
    this.sources.clear();

    if (!sources || sources.length === 0) {
      this.sources.push(this.createSourceControl());
    } else {
      sources.forEach(src => this.sources.push(this.createSourceControl(src)));
    }

    this.sources.markAsPristine();
  }

  private resetWebsiteBlacklist(entries: string[] = []): void {
    this.blockedSites.clear();

    if (!entries || entries.length === 0) {
      this.blockedSites.push(this.createWebsiteControl());
    } else {
      entries.forEach(entry => this.blockedSites.push(this.createWebsiteControl(entry)));
    }

    this.blockedSites.markAsPristine();
  }
}
