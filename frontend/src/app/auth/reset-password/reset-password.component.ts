import { HttpErrorResponse } from '@angular/common/http';
import { Component } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';

import { HttpService } from '../../services/http.service';
import { NotificationService } from '../../services/notification-service.service';
import { ThemeService } from '../../services/theme.service';
import { passwordPolicyMessages, passwordPolicyValidators } from '../password-policy';

@Component({
  selector: 'app-reset-password',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    RouterLink,
    CardModule,
    InputTextModule,
    ButtonModule,
  ],
  templateUrl: './reset-password.component.html',
  styleUrl: '../auth.component.scss'
})
export class ResetPasswordComponent {
  resetPasswordForm: FormGroup;
  token: string;
  readonly passwordRequirements = passwordPolicyMessages();

  constructor(
    private fb: FormBuilder,
    private route: ActivatedRoute,
    private router: Router,
    private http: HttpService,
    private notification: NotificationService,
    protected themeService: ThemeService,
  ) {
    this.token = this.route.snapshot.queryParamMap.get('token')?.trim() ?? '';
    this.resetPasswordForm = this.fb.group({
      password: ['', passwordPolicyValidators()],
      confirmPassword: ['', [Validators.required]],
    });
  }

  passwordsMatch() {
    const { password, confirmPassword } = this.resetPasswordForm.value;
    return password === confirmPassword;
  }

  onSubmit() {
    if (!this.token) {
      this.notification.showError('The password reset link is missing a token.');
      return;
    }

    if (this.resetPasswordForm.invalid || !this.passwordsMatch()) {
      this.resetPasswordForm.markAllAsTouched();
      return;
    }

    const password = this.resetPasswordForm.value.password as string;
    this.http.resetPasswordWithToken({
      token: this.token,
      newPassword: password,
    }).subscribe({
      next: (response) => {
        this.notification.showSuccess(response.message);
        this.router.navigate(['/login']);
      },
      error: (error: HttpErrorResponse) => {
        this.notification.showError(`Could not reset password: ${this.getErrorMessage(error)}`);
      }
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
