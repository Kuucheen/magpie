import { Component, Signal } from '@angular/core';
import { FormBuilder, FormGroup, Validators, AbstractControl, ValidationErrors } from '@angular/forms';

import { ReactiveFormsModule } from '@angular/forms';
import {HttpService} from '../services/http.service';
import {ChangePassword} from '../models/ChangePassword';
import {Button} from 'primeng/button';
import {NotificationService} from '../services/notification-service.service';

import {ThemeService, ThemeName} from '../services/theme.service';
import {Password} from 'primeng/password';
import {DeleteAccount} from '../models/DeleteAccount';
import {UserService} from '../services/authorization/user.service';
import {DialogModule} from 'primeng/dialog';

@Component({
    selector: 'app-account',
  imports: [
    ReactiveFormsModule,
    Button,
    Password,
    DialogModule,
  ],
    templateUrl: './account.component.html',
    styleUrls: ['./account.component.scss']
})
export class AccountComponent {
  passwordForm: FormGroup;
  deleteAccountForm: FormGroup;
  deleteDialogVisible = false;
  readonly themes: ThemeName[];
  readonly currentTheme: Signal<ThemeName>;
  private readonly purpleActivationTarget = 10;
  private purpleActivationCount = 0;
  deletingAccount = false;
  private readonly themeLabels: Record<ThemeName, string> = {
    green: 'Green',
    blue: 'Blue',
    red: 'Red',
    purple: 'Purple'
  };

  private readonly themePreviewColors: Record<ThemeName, string> = {
    green: '#348566',
    blue: '#3b82f6',
    red: '#dc2626',
    purple: '#8b5cf6'
  };

  constructor(private fb: FormBuilder,
              private http: HttpService,
              private themeService: ThemeService,
              private userService: UserService,
              private notification: NotificationService) {
    this.passwordForm = this.fb.group(
      {
        oldPassword: ['', [Validators.required]],
        newPassword: ['', [Validators.required, Validators.minLength(8)]],
        newPasswordCheck: ['', [Validators.required]],
      },
      { validators: this.passwordsMatchValidator }
    );

    this.deleteAccountForm = this.fb.group({
      password: ['', [Validators.required]],
    });

    this.themes = this.themeService.themes;
    this.currentTheme = this.themeService.theme;
  }

  setTheme(theme: ThemeName): void {
    this.themeService.setTheme(theme);
    if (theme === 'purple') {
      this.handlePurpleSecret();
      return;
    }
    this.resetPurpleActivation();
  }

  labelFor(theme: ThemeName): string {
    return this.themeLabels[theme];
  }

  colorFor(theme: ThemeName): string {
    return this.themePreviewColors[theme];
  }

  passwordsMatchValidator(group: AbstractControl): ValidationErrors | null {
    const newPass = group.get('newPassword')?.value;
    const newPassCheck = group.get('newPasswordCheck')?.value;
    return newPass && newPassCheck && newPass === newPassCheck
      ? null
      : { passwordsMismatch: true };
  }

  onSubmit(): void {
    if (this.passwordForm.valid) {

      const changePass: ChangePassword = this.passwordForm.value

      this.http.changePassword(changePass).subscribe({
        next:  res  => this.notification.showInfo(res),
        error: err => this.notification.showError("There has been an error while changing the password! " + err.error.message)
      });

      // this.passwordForm.reset();
    } else {
      this.passwordForm.markAllAsTouched();
    }
  }

  onDeleteAccount(): void {
    if (this.deleteAccountForm.valid) {
      const payload: DeleteAccount = this.deleteAccountForm.value;
      this.deletingAccount = true;

      this.http.deleteAccount(payload).subscribe({
        next: res => {
          this.notification.showSuccess(res);
          this.userService.logoutAndRedirect();
        },
        error: err => {
          this.deletingAccount = false;
          const detail = err?.error?.message ?? err?.error?.error ?? 'Please try again.';
          this.notification.showError("There has been an error while deleting the account! " + detail);
        }
      });
    } else {
      this.deleteAccountForm.markAllAsTouched();
    }
  }

  openDeleteDialog(): void {
    this.deleteDialogVisible = true;
  }

  closeDeleteDialog(): void {
    this.deleteDialogVisible = false;
  }

  onDeleteDialogHide(): void {
    this.deleteDialogVisible = false;
    this.deletingAccount = false;
    this.deleteAccountForm.reset({ password: '' });
  }

  private handlePurpleSecret(): void {
    this.purpleActivationCount += 1;
    const remaining = this.purpleActivationTarget - this.purpleActivationCount;

    if (remaining > 0 && remaining <= 3) {
      this.notification.showInfo(`${remaining}...`);
    }

    if (remaining <= 0) {
      this.resetPurpleActivation();
      this.redirectToGithub();
    }
  }

  private resetPurpleActivation(): void {
    this.purpleActivationCount = 0;
  }

  private redirectToGithub(): void {
    if (typeof globalThis !== 'undefined' && globalThis.location) {
      globalThis.location.href = 'https://github.com/Kuucheen';
    }
  }
}
