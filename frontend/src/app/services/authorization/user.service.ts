import {Injectable, signal} from '@angular/core';
import {HttpService} from '../http.service';
import {Router} from '@angular/router';
import {NotificationService} from '../notification-service.service';
import {AuthInterceptor} from '../auth-interceptor.interceptor';
import {BehaviorSubject} from 'rxjs';

type AuthState = 'checking' | 'authenticated' | 'unauthenticated';

@Injectable({
  providedIn: 'root'
})
export class UserService {
  private static readonly _authState = signal<AuthState>('checking');
  private static role = 'user';
  private static roleSubject = new BehaviorSubject<string | undefined>(undefined);
  public readonly role$ = UserService.roleSubject.asObservable();

  constructor(
    private http: HttpService,
    private router: Router,
    private notification: NotificationService
  ) {
    this.initializeSession();
  }

  private initializeSession() {
    if (typeof window === 'undefined') {
      UserService.setAuthState('unauthenticated');
      return;
    }

    const token = window.localStorage.getItem('magpie-jwt');
    if (token) {
      UserService.setAuthState('checking');
      this.getAndSetRole();
      return;
    }

    UserService.setAuthState('unauthenticated');
  }

  public getAndSetRole() {
    if (UserService.authState() !== 'authenticated') {
      UserService.setAuthState('checking');
    }
    this.http.getUserRole().subscribe({
      next: res => {
        UserService.setAuthState('authenticated');
        UserService.setRole(res);
      },
      error: err => {
        if (err.status && err.status !== 401 && err.status !== 403) {
          this.notification.showError("Error while getting user role! " + err.error.message)
        }
        if (err.status === 401 || err.status === 403) {
          this.logoutAndRedirect();
          return;
        }
        if (!err.status || err.status === 0) {
          // If the backend cannot be reached we keep the token but fall back to the login view.
          UserService.setAuthState('unauthenticated');
          UserService.setRole('user');
          return;
        }
        UserService.setAuthState('unauthenticated');
        UserService.setRole('user');
      }
    })
  }

  public static isLoggedIn() {
    return UserService._authState() === 'authenticated';
  }

  public static setLoggedIn(loggedIn: boolean) {
    UserService.setAuthState(loggedIn ? 'authenticated' : 'unauthenticated');
  }

  public static setRole(role: string) {
    UserService.role = role;
    UserService.roleSubject.next(role);
  }

  public static isAdmin() {
    return UserService.role === 'admin';
  }

  public static logout() {
    if (typeof window !== 'undefined') {
      window.localStorage.removeItem('magpie-jwt');
    }
    AuthInterceptor.setToken('');
    UserService.setLoggedIn(false);
    UserService.setRole('user');
  }

  public logoutAndRedirect() {
    UserService.logout()
    this.router.navigate(['/login']);
  }

  public static authState() {
    return UserService._authState();
  }

  private static setAuthState(state: AuthState) {
    UserService._authState.set(state);
  }

}
