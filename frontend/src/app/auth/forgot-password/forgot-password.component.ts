import { HttpErrorResponse } from '@angular/common/http';
import { Component, signal } from '@angular/core';
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
export class ForgotPasswordComponent {
  forgotPasswordForm: FormGroup;
  submitted = signal(false);

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

  onSubmit() {
    if (this.forgotPasswordForm.invalid) {
      this.forgotPasswordForm.markAllAsTouched();
      return;
    }

    const email = this.forgotPasswordForm.value.email as string;
    this.http.requestPasswordReset({ email }).subscribe({
      next: (response) => {
        this.submitted.set(true);
        this.notification.showSuccess(response.message);
      },
      error: (error: HttpErrorResponse) => {
        this.notification.showError(`Could not send password reset email: ${this.getErrorMessage(error)}`);
      },
    });
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
