import {Injectable} from '@angular/core';
import { ActivatedRouteSnapshot, CanActivate, Router, RouterStateSnapshot, UrlTree } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError, map, tap } from 'rxjs/operators';
import { HttpService } from '../http.service';
import { UserService } from './user.service';

@Injectable({
  providedIn: 'root'
})
export class AuthLoginGuardService implements CanActivate {
  constructor(
    private http: HttpService,
    private router: Router,
    private userService: UserService
  ) {}

  canActivate(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<boolean | UrlTree> {
    const token = typeof window !== 'undefined'
      ? window.localStorage.getItem('magpie-jwt')
      : null;
    const authState = UserService.authState();

    if (!token || authState === 'unauthenticated') {
      return of(true);
    }

    const returnUrl = typeof window !== 'undefined'
      ? window.sessionStorage.getItem('magpie-return-url')
      : null;

    if (authState === 'authenticated') {
      const target = returnUrl && returnUrl.trim().length > 0 ? returnUrl : '/';

      if (typeof window !== 'undefined') {
        window.sessionStorage.removeItem('magpie-return-url');
      }

      return of(this.router.parseUrl(target));
    }

    return this.http.checkLogin().pipe(
      tap(() => {
        UserService.setLoggedIn(true);
        this.userService.getAndSetRole();
      }),
      map(() => {
        const target = returnUrl && returnUrl.trim().length > 0 ? returnUrl : '/';

        if (typeof window !== 'undefined') {
          window.sessionStorage.removeItem('magpie-return-url');
        }

        return this.router.parseUrl(target);
      }),
      catchError(() => {
        UserService.setLoggedIn(false);
        return of(true);
      })
    );
  }
}
