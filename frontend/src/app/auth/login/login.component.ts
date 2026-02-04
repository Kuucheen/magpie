import { Component, OnDestroy, model, signal } from '@angular/core';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule, FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';
import { CheckboxModule } from 'primeng/checkbox';

import { User } from '../../models/UserModel';
import { HttpService } from '../../services/http.service';
import { UserService } from '../../services/authorization/user.service';
import { AuthInterceptor } from '../../services/auth-interceptor.interceptor';
import {NotificationService} from '../../services/notification-service.service';
import {ThemeService} from '../../services/theme.service';
import {LoadingComponent} from '../../ui-elements/loading/loading.component';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    FormsModule,
    RouterLink,
    CardModule,
    InputTextModule,
    ButtonModule,
    CheckboxModule,
    LoadingComponent,
  ],
  templateUrl: './login.component.html',
  styleUrl: '../auth.component.scss'
})
export class LoginComponent implements OnDestroy {
  loginForm: FormGroup;
  rememberPass = model(false);
  shouldRemember = false;
  autoLoginChecking = signal(false);
  private autoLoginTimeoutId?: ReturnType<typeof setTimeout>;
  private autoLoginIntervalId?: ReturnType<typeof setInterval>;

  constructor(
    private fb: FormBuilder,
    private http: HttpService,
    private router: Router,
    private route: ActivatedRoute,
    protected themeService: ThemeService,
    private notification: NotificationService
  ) {
    this.loginForm = this.fb.group({
      email: ['', [Validators.required, Validators.email]],
      password: ['', [Validators.required, Validators.minLength(6)]],
    });

    this.rememberPass.subscribe(res => (this.shouldRemember = res));

    const returnUrl = this.route.snapshot.queryParamMap.get('returnUrl');
    const isBrowser = typeof window !== 'undefined';
    const hasToken = isBrowser && !!window.localStorage.getItem('magpie-jwt');
    if (returnUrl) {
      this.autoLoginChecking.set(true);
    } else {
      this.autoLoginChecking.set(false);
    }
    if (returnUrl && hasToken) {
      this.startAutoLoginWatcher();
    } else if (returnUrl && isBrowser && !hasToken) {
      this.autoLoginChecking.set(false);
    }
  }

  ngOnDestroy(): void {
    this.clearAutoLoginTimers();
  }

  onLogin() {
    const { email, password } = this.loginForm.value;
    const user: User = { email, password };

    this.http.loginUser(user).subscribe({
      next: (response) => {
        if (this.shouldRemember) {
          localStorage.setItem('magpie-jwt', response.token);
        } else {
          localStorage.removeItem('magpie-jwt');
        }
        AuthInterceptor.setToken(response.token);
        UserService.setLoggedIn(true);
        UserService.setRole(response.role);
        const returnUrl = typeof window !== 'undefined'
          ? window.sessionStorage.getItem('magpie-return-url')
          : null;

        if (typeof window !== 'undefined') {
          window.sessionStorage.removeItem('magpie-return-url');
        }

        const target = returnUrl && returnUrl.trim().length > 0 ? returnUrl : '/';
        this.router.navigateByUrl(target);
      },
      error: (err) => {
        UserService.setLoggedIn(false);
        if (err.status === 401) {
          this.notification.showError('Username or Password is incorrect');
        } else {
          this.notification.showError('Something went wrong while login! Error code: ' + err.status)
        }
      },
    });
  }

  private startAutoLoginWatcher(): void {
    const timeoutMs = 10000;
    const intervalMs = 50;
    const start = Date.now();

    this.autoLoginIntervalId = setInterval(() => {
      const state = UserService.authState();
      if (state !== 'checking') {
        if (state === 'unauthenticated') {
          this.autoLoginChecking.set(false);
        }
        this.clearAutoLoginTimers();
        return;
      }

      if (Date.now() - start >= timeoutMs) {
        this.autoLoginChecking.set(false);
        this.clearAutoLoginTimers();
      }
    }, intervalMs);

    this.autoLoginTimeoutId = setTimeout(() => {
      this.autoLoginChecking.set(false);
      this.clearAutoLoginTimers();
    }, timeoutMs);
  }

  private clearAutoLoginTimers(): void {
    if (this.autoLoginIntervalId) {
      clearInterval(this.autoLoginIntervalId);
      this.autoLoginIntervalId = undefined;
    }
    if (this.autoLoginTimeoutId) {
      clearTimeout(this.autoLoginTimeoutId);
      this.autoLoginTimeoutId = undefined;
    }
  }
}
