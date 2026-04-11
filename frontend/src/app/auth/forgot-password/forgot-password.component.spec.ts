import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { of, throwError } from 'rxjs';

import { ForgotPasswordComponent } from './forgot-password.component';
import { HttpService } from '../../services/http.service';
import { NotificationService } from '../../services/notification-service.service';
import { ThemeService } from '../../services/theme.service';

describe('ForgotPasswordComponent', () => {
  let component: ForgotPasswordComponent;
  let fixture: ComponentFixture<ForgotPasswordComponent>;
  let requestPasswordResetSpy: jasmine.Spy;
  let showSuccessSpy: jasmine.Spy;
  let showErrorSpy: jasmine.Spy;

  beforeEach(async () => {
    requestPasswordResetSpy = jasmine.createSpy('requestPasswordReset');
    showSuccessSpy = jasmine.createSpy('showSuccess');
    showErrorSpy = jasmine.createSpy('showError');

    await TestBed.configureTestingModule({
      imports: [ForgotPasswordComponent, RouterTestingModule],
      providers: [
        {
          provide: HttpService,
          useValue: {
            requestPasswordReset: requestPasswordResetSpy,
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

    fixture = TestBed.createComponent(ForgotPasswordComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should show a success message after requesting a reset link', () => {
    requestPasswordResetSpy.and.returnValue(of({ message: 'If an account exists for that email, a password reset link has been sent.' }));
    component.forgotPasswordForm.setValue({ email: 'user@example.com' });

    component.onSubmit();

    expect(showSuccessSpy).toHaveBeenCalled();
    expect(component.submitted()).toBeTrue();
    expect(component.cooldownSeconds()).toBe(60);
  });

  it('should show the backend error detail when reset requests fail', () => {
    requestPasswordResetSpy.and.returnValue(
      throwError(() => new HttpErrorResponse({
        status: 503,
        error: { error: 'Password recovery is not configured' },
      }))
    );
    component.forgotPasswordForm.setValue({ email: 'user@example.com' });

    component.onSubmit();

    expect(showErrorSpy).toHaveBeenCalledWith('Could not send password reset email: Password recovery is not configured');
  });

  it('should start a countdown from Retry-After on rate limits', () => {
    requestPasswordResetSpy.and.returnValue(
      throwError(() => new HttpErrorResponse({
        status: 429,
        headers: new HttpHeaders({ 'Retry-After': '42' }),
        error: { error: 'Too many password reset requests. Please try again later.' },
      }))
    );
    component.forgotPasswordForm.setValue({ email: 'user@example.com' });

    component.onSubmit();

    expect(component.cooldownSeconds()).toBe(42);
    expect(component.sendButtonLabel()).toBe('42s');
  });
});
