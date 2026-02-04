import { Injectable } from '@angular/core';
import {UserService} from './user.service';
import {ActivatedRouteSnapshot, CanActivate, CanActivateChild, Router, RouterStateSnapshot, UrlTree} from '@angular/router';

@Injectable({
  providedIn: 'root'
})
export class AuthGuardService implements CanActivate, CanActivateChild {

  constructor(private router: Router) { }

  canActivate(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): boolean | UrlTree | Promise<boolean | UrlTree> {
    const returnUrl = state.url || '';
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
          if (state === 'authenticated') {
            resolve(true);
            return;
          }
          resolve(this.router.createUrlTree(['login'], {
            queryParams: returnUrl ? { returnUrl } : undefined,
          }));
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
