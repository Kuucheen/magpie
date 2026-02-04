import {Component, OnDestroy, OnInit} from '@angular/core';

import {FormArray, FormBuilder, FormGroup, ReactiveFormsModule} from '@angular/forms';
import {TooltipComponent} from '../../tooltip/tooltip.component';
import {InputText} from 'primeng/inputtext';
import {Button} from 'primeng/button';
import {SettingsService} from '../../services/settings.service';
import {NotificationService} from '../../services/notification-service.service';
import {UserSettings} from '../../models/UserSettings';
import {Subject} from 'rxjs';
import {filter, takeUntil} from 'rxjs/operators';

@Component({
  selector: 'app-checker-judges',
  standalone: true,
  imports: [ReactiveFormsModule, TooltipComponent, InputText, Button],
  templateUrl: './checker-judges.component.html',
  styleUrls: ['./checker-judges.component.scss']
})
export class CheckerJudgesComponent implements OnInit, OnDestroy {
  judgesForm: FormArray<FormGroup>;
  private destroy$ = new Subject<void>();

  constructor(
    private fb: FormBuilder,
    private settingsService: SettingsService,
    private notification: NotificationService
  ) {
    this.judgesForm = this.fb.array<FormGroup>([]);
  }

  ngOnInit(): void {
    this.populateJudges(this.settingsService.getUserSettings());

    this.settingsService.userSettings$
      .pipe(
        filter((settings): settings is UserSettings => !!settings),
        takeUntil(this.destroy$)
      )
      .subscribe(settings => this.populateJudges(settings));
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  get judgeControls(): FormGroup[] {
    return this.judgesForm.controls as FormGroup[];
  }

  addJudge(): void {
    this.judgesForm.push(this.createJudgeGroup('', 'default'));
    this.judgesForm.markAsDirty();
  }

  removeJudge(index: number): void {
    if (index < 0 || index >= this.judgesForm.length) {
      return;
    }

    this.judgesForm.removeAt(index);
    this.judgesForm.markAsDirty();
  }

  onSubmit(): void {
    const current = this.settingsService.getUserSettings();
    const payload = {
      HTTPProtocol: current?.http_protocol ?? false,
      HTTPSProtocol: current?.https_protocol ?? true,
      SOCKS4Protocol: current?.socks4_protocol ?? false,
      SOCKS5Protocol: current?.socks5_protocol ?? false,
      Timeout: current?.timeout ?? 7500,
      Retries: current?.retries ?? 2,
      UseHttpsForSocks: current?.UseHttpsForSocks ?? true,
      TransportProtocol: current?.transport_protocol ?? 'tcp',
      judges: this.judgesForm.value
    };

    this.settingsService.saveUserSettings(payload).subscribe({
      next: (resp) => {
        this.notification.showSuccess(resp.message);
        this.populateJudges(this.settingsService.getUserSettings());
      },
      error: (err) => {
        console.error('Error saving judges:', err);
        const reason = err?.error?.message ?? err?.error?.error ?? 'Failed to save settings!';
        this.notification.showError(reason);
      }
    });
  }

  private populateJudges(settings: UserSettings | undefined): void {
    this.judgesForm.clear();

    if (settings?.judges?.length) {
      settings.judges.forEach(judge => this.judgesForm.push(this.createJudgeGroup(judge.url, judge.regex)));
    }

    if (this.judgesForm.length === 0) {
      this.judgesForm.push(this.createJudgeGroup());
    }

    this.judgesForm.markAsPristine();
  }

  private createJudgeGroup(url: string = '', regex: string = ''): FormGroup {
    return this.fb.group({
      url: [url],
      regex: [regex]
    });
  }
}
