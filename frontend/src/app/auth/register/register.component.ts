import { Component } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';

import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';

import { HttpService } from '../../services/http.service';
import { User } from '../../models/UserModel';
import { UserService } from '../../services/authorization/user.service';
import { AuthInterceptor } from '../../services/auth-interceptor.interceptor';
import {NotificationService} from '../../services/notification-service.service';
import {ThemeService} from '../../services/theme.service';

@Component({
  selector: 'app-register',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    RouterLink,
    CardModule,
    InputTextModule,
    ButtonModule
  ],
  templateUrl: './register.component.html',
  styleUrl: '../auth.component.scss'
})
export class RegisterComponent {
  registerForm: FormGroup;

  constructor(
    private fb: FormBuilder,
    private http: HttpService,
    private router: Router,
    private user: UserService,
    protected themeService: ThemeService
  ) {
    this.registerForm = this.fb.group({
      email: ['', [Validators.required, Validators.email]],
      password: ['', [Validators.required, Validators.minLength(8)]],
      confirmPassword: ['', [Validators.required]]
    });
  }

  onRegister() {
    if (this.registerForm.valid) {
      const { email, password, confirmPassword } = this.registerForm.value;

      if (!this.passwordIsTheSame() || password.length < 8) {
        return;
      }

      const user: User = { email, password };

      this.http.registerUser(user).subscribe({
        next: (response) => {
          AuthInterceptor.setToken(response.token);
          UserService.setLoggedIn(true);
          this.user.getAndSetRole();
          NotificationService.showSuccess('Registration successful');
          this.router.navigate(['/']);
        },
        error: (error) =>
          NotificationService.showError('Registration failed: ' + error.error.error)
      });
    }
  }

  passwordIsTheSame() {
    const { password, confirmPassword } = this.registerForm.value;
    return password === confirmPassword;
  }
}
