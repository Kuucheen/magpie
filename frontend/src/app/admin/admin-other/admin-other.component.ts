import {Component, OnDestroy, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, ReactiveFormsModule} from '@angular/forms';
import {SettingsService} from '../../services/settings.service';
import {GlobalSettings} from '../../models/GlobalSettings';
import {Subject} from 'rxjs';
import {filter, takeUntil} from 'rxjs/operators';

import {ButtonModule} from 'primeng/button';
import {CheckboxModule} from 'primeng/checkbox';
import {DividerModule} from 'primeng/divider';
import {InputTextModule} from 'primeng/inputtext';
import {SelectModule} from 'primeng/select';
import {NotificationService} from '../../services/notification-service.service';
import {Message} from 'primeng/message';

@Component({
  selector: 'app-admin-other',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    ButtonModule,
    CheckboxModule,
    DividerModule,
    InputTextModule,
    SelectModule,
    Message
  ],
  templateUrl: './admin-other.component.html',
  styleUrl: './admin-other.component.scss'
})
export class AdminOtherComponent implements OnInit, OnDestroy {
  daysList = Array.from({ length: 31 }, (_, i) => ({ label: `${i} Days`, value: i }));
  hoursList = Array.from({ length: 24 }, (_, i) => ({ label: `${i} Hours`, value: i }));
  minutesList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Minutes`, value: i }));
  secondsList = Array.from({ length: 60 }, (_, i) => ({ label: `${i} Seconds`, value: i }));
  form: FormGroup;
  lastUpdatedLabel = 'Never';
  private destroy$ = new Subject<void>();

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private notification: NotificationService
  ) {
    this.form = this.fb.group({
      api_key: [''],
      auto_update: [false],
      update_timer: this.fb.group({
        days: [1],
        hours: [0],
        minutes: [0],
        seconds: [0]
      }),
      last_updated_at: [null]
    });
  }

  ngOnInit(): void {
    this.settingsService.settings$
      .pipe(
        filter((settings): settings is GlobalSettings => !!settings),
        takeUntil(this.destroy$)
      )
      .subscribe(settings => this.updateFormWithSettings(settings));

    this.form.get('auto_update')?.valueChanges
      .pipe(takeUntil(this.destroy$))
      .subscribe((enabled: boolean) => this.toggleTimerControls(enabled));

    this.toggleTimerControls(this.form.get('auto_update')?.value ?? false);
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  onSubmit(): void {
    if (this.form.invalid) {
      return;
    }

    const raw = this.form.getRawValue();
    const timer = raw.update_timer ?? {};
    const payload = {
      geolite: {
        api_key: typeof raw.api_key === 'string' ? raw.api_key.trim() : '',
        auto_update: !!raw.auto_update,
        update_timer: {
          days: timer.days ?? 1,
          hours: timer.hours ?? 0,
          minutes: timer.minutes ?? 0,
          seconds: timer.seconds ?? 0
        },
        last_updated_at: raw.last_updated_at ?? null
      }
    };

    this.settingsService.saveGlobalSettings(payload).subscribe({
      next: (resp) => {
        this.notification.showSuccess(resp.message ?? 'Settings saved');
        this.form.markAsPristine();
      },
      error: (err) => {
        console.error('Error saving GeoLite settings:', err);
        this.notification.showError('Failed to save GeoLite settings: ' + (err?.error?.message ?? 'Unknown error'));
      }
    });
  }

  get updateTimerGroup(): FormGroup | null {
    return this.form.get('update_timer') as FormGroup | null;
  }

  private updateFormWithSettings(settings: GlobalSettings): void {
    const geolite = settings.geolite;
    this.form.patchValue({
      api_key: geolite?.api_key ?? '',
      auto_update: geolite?.auto_update ?? false,
      update_timer: {
        days: geolite?.update_timer.days ?? 1,
        hours: geolite?.update_timer.hours ?? 0,
        minutes: geolite?.update_timer.minutes ?? 0,
        seconds: geolite?.update_timer.seconds ?? 0
      },
      last_updated_at: geolite?.last_updated_at ?? null
    }, { emitEvent: false });

    this.form.markAsPristine();
    this.lastUpdatedLabel = this.formatTimestamp(geolite?.last_updated_at);
    this.toggleTimerControls(geolite?.auto_update ?? false);
  }

  private toggleTimerControls(enabled: boolean): void {
    const group = this.updateTimerGroup;
    if (!group) {
      return;
    }

    if (enabled) {
      group.enable({ emitEvent: false });
    } else {
      group.disable({ emitEvent: false });
    }
  }

  private formatTimestamp(timestamp?: string | null): string {
    if (!timestamp) {
      return 'Never';
    }

    const parsed = new Date(timestamp);
    if (isNaN(parsed.getTime())) {
      return timestamp;
    }

    return parsed.toLocaleString();
  }
}
