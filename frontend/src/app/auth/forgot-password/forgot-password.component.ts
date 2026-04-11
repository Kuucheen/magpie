import { HttpErrorResponse } from '@angular/common/http';
import { Component, OnDestroy, signal } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';

import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';

import { HttpService } from '../../services/http.service';
import { NotificationService } from '../../services/notification-service.service';
import { ThemeService } from '../../services/theme.service';

@Component({
  selector: 'app-forgot-password',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    RouterLink,
    CardModule,
    InputTextModule,
    ButtonModule,
  ],
  templateUrl: './forgot-password.component.html',
  styleUrl: '../auth.component.scss'
})
export class ForgotPasswordComponent implements OnDestroy {
  forgotPasswordForm: FormGroup;
  submitted = signal(false);
  sending = signal(false);
  cooldownSeconds = signal(0);
  private cooldownIntervalId?: ReturnType<typeof setInterval>;

  constructor(
    private fb: FormBuilder,
    private http: HttpService,
    private notification: NotificationService,
    protected themeService: ThemeService,
  ) {
    this.forgotPasswordForm = this.fb.group({
      email: ['', [Validators.required, Validators.email]],
    });
  }

  ngOnDestroy(): void {
    this.clearCooldownTimer();
  }

  onSubmit() {
    if (this.forgotPasswordForm.invalid || this.cooldownSeconds() > 0 || this.sending()) {
      this.forgotPasswordForm.markAllAsTouched();
      return;
    }

    this.sending.set(true);
    const email = this.forgotPasswordForm.value.email as string;
    this.http.requestPasswordReset({ email }).subscribe({
      next: (response) => {
        this.sending.set(false);
        this.submitted.set(true);
        this.startCooldown(60);
        this.notification.showSuccess(response.message);
      },
      error: (error: HttpErrorResponse) => {
        this.sending.set(false);
        if (error.status === 429) {
          this.startCooldown(this.getRetryAfterSeconds(error));
        }
        this.notification.showError(`Could not send password reset email: ${this.getErrorMessage(error)}`);
      },
    });
  }

  sendButtonLabel(): string {
    if (this.sending()) {
      return 'Sending...';
    }
    if (this.cooldownSeconds() > 0) {
      return `${this.cooldownSeconds()}s`;
    }
    return 'Send';
  }

  private startCooldown(seconds: number): void {
    const normalizedSeconds = Math.max(1, Math.floor(seconds));
    this.clearCooldownTimer();
    this.cooldownSeconds.set(normalizedSeconds);

    this.cooldownIntervalId = setInterval(() => {
      const nextValue = this.cooldownSeconds() - 1;
      if (nextValue <= 0) {
        this.clearCooldownTimer();
        this.cooldownSeconds.set(0);
        return;
      }
      this.cooldownSeconds.set(nextValue);
    }, 1000);
  }

  private clearCooldownTimer(): void {
    if (!this.cooldownIntervalId) {
      return;
    }
    clearInterval(this.cooldownIntervalId);
    this.cooldownIntervalId = undefined;
  }

  private getRetryAfterSeconds(error: HttpErrorResponse): number {
    const raw = error.headers?.get('Retry-After')?.trim();
    const parsed = raw ? Number.parseInt(raw, 10) : NaN;
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed;
    }
    return 60;
  }

  private getErrorMessage(error: HttpErrorResponse): string {
    const apiError = error.error;

    if (typeof apiError === 'string' && apiError.trim().length > 0) {
      return apiError;
    }

    if (apiError && typeof apiError === 'object') {
      const structuredError = apiError as { error?: unknown; message?: unknown };
      if (typeof structuredError.error === 'string' && structuredError.error.trim().length > 0) {
        return structuredError.error;
      }
      if (typeof structuredError.message === 'string' && structuredError.message.trim().length > 0) {
        return structuredError.message;
      }
    }

    if (error.status === 0) {
      return 'Unable to reach the server';
    }

    return error.message?.trim() || `Request failed with status ${error.status}`;
  }
}
