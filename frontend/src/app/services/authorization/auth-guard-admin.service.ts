import { Injectable } from '@angular/core';
import {ActivatedRouteSnapshot, CanActivate, CanActivateChild, Router, RouterStateSnapshot, UrlTree} from '@angular/router';
import {UserService} from './user.service';

@Injectable({
  providedIn: 'root'
})
export class AuthGuardAdminService implements CanActivate, CanActivateChild {

  constructor(private router: Router) { }

  canActivate(route?: ActivatedRouteSnapshot, state?: RouterStateSnapshot): boolean | UrlTree | Promise<boolean | UrlTree> {
    const returnUrl = state?.url ?? '';
    const hasToken = typeof window !== 'undefined' && !!window.localStorage.getItem('magpie-jwt');

    if (UserService.authState() === 'checking') {
      this.storeReturnUrl(returnUrl);
      return this.waitForAuthResolution(returnUrl);
    }

    if (!UserService.isLoggedIn()) {
      if (hasToken) {
        UserService.setChecking();
        this.storeReturnUrl(returnUrl);
        return this.waitForAuthResolution(returnUrl);
      }

      this.storeReturnUrl(returnUrl);
      return this.router.createUrlTree(['login'], {
        queryParams: returnUrl ? { returnUrl } : undefined,
      });
    }

    if (!UserService.isAdmin()) {
      return this.router.createUrlTree(['/']);
    }

    return true;
  }

  canActivateChild(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): boolean | UrlTree | Promise<boolean | UrlTree> {
    return this.canActivate(route, state);
  }

  private storeReturnUrl(returnUrl: string): void {
    if (typeof window !== 'undefined' && returnUrl) {
      window.sessionStorage.setItem('magpie-return-url', returnUrl);
    }
  }

  private waitForAuthResolution(returnUrl: string): Promise<boolean | UrlTree> {
    const timeoutMs = 10000;
    const intervalMs = 50;
    const start = Date.now();

    return new Promise(resolve => {
      const tick = () => {
        const state = UserService.authState();
        if (state !== 'checking') {
          if (state !== 'authenticated') {
            resolve(this.router.createUrlTree(['login'], {
              queryParams: returnUrl ? { returnUrl } : undefined,
            }));
            return;
          }

          resolve(UserService.isAdmin() ? true : this.router.createUrlTree(['/']));
          return;
        }

        if (Date.now() - start >= timeoutMs) {
          resolve(this.router.createUrlTree(['login'], {
            queryParams: returnUrl ? { returnUrl } : undefined,
          }));
          return;
        }

        setTimeout(tick, intervalMs);
      };

      tick();
    });
  }
}
