import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { throwError } from 'rxjs';

import { RegisterComponent } from './register.component';
import { HttpService } from '../../services/http.service';
import { NotificationService } from '../../services/notification-service.service';
import { ThemeService } from '../../services/theme.service';
import { UserService } from '../../services/authorization/user.service';
import { passwordMinLength } from '../password-policy';

describe('RegisterComponent', () => {
  let component: RegisterComponent;
  let fixture: ComponentFixture<RegisterComponent>;
  let registerUserSpy: jasmine.Spy;
  let showErrorSpy: jasmine.Spy;

  beforeEach(async () => {
    registerUserSpy = jasmine.createSpy('registerUser');
    showErrorSpy = jasmine.createSpy('showError');

    await TestBed.configureTestingModule({
      imports: [RegisterComponent, RouterTestingModule],
      providers: [
        {
          provide: HttpService,
          useValue: {
            registerUser: registerUserSpy,
          },
        },
        {
          provide: UserService,
          useValue: {
            getAndSetRole: jasmine.createSpy('getAndSetRole'),
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
            showError: showErrorSpy,
            showSuccess: jasmine.createSpy('showSuccess'),
          },
        },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(RegisterComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should show the backend registration error detail', () => {
    registerUserSpy.and.returnValue(
      throwError(() => new HttpErrorResponse({
        status: 409,
        error: { error: 'Email already in use' },
      }))
    );
    component.registerForm.setValue({
      email: 'test@example.com',
      password: 'StrongPassword123',
      confirmPassword: 'StrongPassword123',
    });

    component.onRegister();

    expect(showErrorSpy).toHaveBeenCalledWith('Registration failed: Email already in use');
  });

  it('should show a friendly message when the server cannot be reached', () => {
    registerUserSpy.and.returnValue(
      throwError(() => new HttpErrorResponse({
        status: 0,
        error: new ProgressEvent('error'),
      }))
    );
    component.registerForm.setValue({
      email: 'test@example.com',
      password: 'StrongPassword123',
      confirmPassword: 'StrongPassword123',
    });

    component.onRegister();

    expect(showErrorSpy).toHaveBeenCalledWith('Registration failed: Unable to reach the server');
  });

  it('should reject weak passwords before calling the backend', () => {
    component.registerForm.setValue({
      email: 'test@example.com',
      password: 'weakpass',
      confirmPassword: 'weakpass',
    });

    component.onRegister();

    expect(registerUserSpy).not.toHaveBeenCalled();
    expect(component.registerForm.get('password')?.hasError('minlength')).toBeTrue();
    expect(passwordMinLength).toBe(12);
  });
});
