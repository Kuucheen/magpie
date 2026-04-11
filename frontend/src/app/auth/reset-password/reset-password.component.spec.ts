import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { of, throwError } from 'rxjs';

import { ResetPasswordComponent } from './reset-password.component';
import { HttpService } from '../../services/http.service';
import { NotificationService } from '../../services/notification-service.service';
import { ThemeService } from '../../services/theme.service';
import { passwordMinLength } from '../password-policy';

describe('ResetPasswordComponent', () => {
  let component: ResetPasswordComponent;
  let fixture: ComponentFixture<ResetPasswordComponent>;
  let resetPasswordWithTokenSpy: jasmine.Spy;
  let showSuccessSpy: jasmine.Spy;
  let showErrorSpy: jasmine.Spy;

  beforeEach(async () => {
    resetPasswordWithTokenSpy = jasmine.createSpy('resetPasswordWithToken');
    showSuccessSpy = jasmine.createSpy('showSuccess');
    showErrorSpy = jasmine.createSpy('showError');

    await TestBed.configureTestingModule({
      imports: [ResetPasswordComponent, RouterTestingModule],
      providers: [
        {
          provide: HttpService,
          useValue: {
            resetPasswordWithToken: resetPasswordWithTokenSpy,
          },
        },
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              queryParamMap: new Map([['token', 'reset-token']]),
            },
          },
        },
        {
          provide: ThemeService,
          useValue: {
            theme: () => 'green',
          },
        },
        {
          provide: NotificationService,
          useValue: {
            showSuccess: showSuccessSpy,
            showError: showErrorSpy,
          },
        },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(ResetPasswordComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should submit the token and new password', () => {
    resetPasswordWithTokenSpy.and.returnValue(of({ message: 'Password reset successfully' }));
    component.resetPasswordForm.setValue({
      password: 'StrongPassword123',
      confirmPassword: 'StrongPassword123',
    });

    component.onSubmit();

    expect(resetPasswordWithTokenSpy).toHaveBeenCalledWith({
      token: 'reset-token',
      newPassword: 'StrongPassword123',
    });
    expect(showSuccessSpy).toHaveBeenCalledWith('Password reset successfully');
  });

  it('should show backend reset errors', () => {
    resetPasswordWithTokenSpy.and.returnValue(
      throwError(() => new HttpErrorResponse({
        status: 401,
        error: { error: 'Invalid or expired password reset token' },
      }))
    );
    component.resetPasswordForm.setValue({
      password: 'StrongPassword123',
      confirmPassword: 'StrongPassword123',
    });

    component.onSubmit();

    expect(showErrorSpy).toHaveBeenCalledWith('Could not reset password: Invalid or expired password reset token');
  });

  it('should reject weak passwords before submitting', () => {
    component.resetPasswordForm.setValue({
      password: 'weakpass',
      confirmPassword: 'weakpass',
    });

    component.onSubmit();

    expect(resetPasswordWithTokenSpy).not.toHaveBeenCalled();
    expect(component.resetPasswordForm.get('password')?.hasError('minlength')).toBeTrue();
    expect(passwordMinLength).toBe(12);
  });
});
