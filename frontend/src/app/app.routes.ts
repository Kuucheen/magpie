import { Routes } from '@angular/router';
import {DashboardComponent} from './dashboard/dashboard.component';
import {ProxiesComponent} from './proxies/proxies.component';
import {ProxyDetailComponent} from './proxies/proxy-detail/proxy-detail.component';
import {RegisterComponent} from './auth/register/register.component';
import {LoginComponent} from './auth/login/login.component';
import {AuthGuardService} from './services/authorization/auth-guard.service';
import {AccountComponent} from './account/account.component';
import {AuthGuardAdminService} from './services/authorization/auth-guard-admin.service';
import {AuthLoginGuardService} from './services/authorization/auth-login-guard.service';
import {AddProxiesComponent} from './proxies/proxy-list/add-proxies/add-proxies.component';
import {CheckerJudgesComponent} from './checker/judges/checker-judges.component';
import {CheckerSettingsComponent} from './checker/settings/checker-settings.component';
import {RotatingProxiesComponent} from './rotating-proxies/rotating-proxies.component';
import {AdminCheckerComponent} from './admin/admin-checker/admin-checker.component';
import {AdminScraperComponent} from './admin/admin-scraper/admin-scraper.component';
import {AdminOtherComponent} from './admin/admin-other/admin-other.component';
import {AdminBlacklistComponent} from './admin/admin-blacklist/admin-blacklist.component';
import {ScraperComponent} from './scraper/scraper.component';
import {ScrapeSourceDetailComponent} from './scraper/scrape-source-detail/scrape-source-detail.component';
import {NotificationsComponent} from './notifications/notifications.component';

export const routes: Routes = [
  {path: "account", component: AccountComponent, canActivate: [AuthGuardService]},
  {path: "addProxies", component: AddProxiesComponent, canActivate: [AuthGuardService]},
  {path: "rotating", component: RotatingProxiesComponent, canActivate: [AuthGuardService]},
  {path: "proxies", component: ProxiesComponent, canActivate: [AuthGuardService]},
  {path: "proxies/:id", component: ProxyDetailComponent, canActivate: [AuthGuardService]},
  {path: "scraper/:id", component: ScrapeSourceDetailComponent, canActivate: [AuthGuardService]},
  {path: "scraper", component: ScraperComponent, canActivate: [AuthGuardService]},
  {path: "checker", redirectTo: "checker/settings", pathMatch: "full"},
  {path: "checker/settings", component: CheckerSettingsComponent, canActivate: [AuthGuardService]},
  {path: "checker/judges", component: CheckerJudgesComponent, canActivate: [AuthGuardService]},
  {path: "global/checker", component: AdminCheckerComponent, canActivate: [AuthGuardAdminService]},
  {path: "global/scraper", component: AdminScraperComponent, canActivate: [AuthGuardAdminService]},
  {path: "global/blacklist", component: AdminBlacklistComponent, canActivate: [AuthGuardAdminService]},
  {path: "global/other", component: AdminOtherComponent, canActivate: [AuthGuardAdminService]},
  {path: "notifications", component: NotificationsComponent, canActivate: [AuthGuardService]},
  {path: "register", component: RegisterComponent, canActivate: [AuthLoginGuardService]},
  {path: "login", component: LoginComponent, canActivate: [AuthLoginGuardService]},
  {path: "**", component: DashboardComponent, canActivate: [AuthGuardService]}
];
