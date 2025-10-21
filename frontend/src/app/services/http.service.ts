import { Injectable } from '@angular/core';
import {environment} from '../../environments/environment';
import {HttpClient, HttpParams} from '@angular/common/http';
import {User} from '../models/UserModel';
import {jwtToken} from '../models/JwtToken';
import {ProxyPage} from '../models/ProxyInfo';
import {GlobalSettings} from '../models/GlobalSettings';
import {UserSettings} from '../models/UserSettings';
import {ExportSettings} from '../models/ExportSettings';
import {ScrapeSourceInfo} from '../models/ScrapeSourceInfo';
import {DashboardInfo} from '../models/DashboardInfo';
import {ChangePassword} from '../models/ChangePassword';
import {ProxyDetail} from '../models/ProxyDetail';
import {ProxyStatistic} from '../models/ProxyStatistic';
import {ProxyStatisticResponseDetail} from '../models/ProxyStatisticResponseDetail';
import {RotatingProxy, CreateRotatingProxy, RotatingProxyNext} from '../models/RotatingProxy';
import {map} from 'rxjs/operators';
import {DeleteSettings} from '../models/DeleteSettings';

@Injectable({
  providedIn: 'root'
})
export class HttpService {
  private apiUrl = environment.apiUrl;

  constructor(private http: HttpClient) { }

  checkLogin() {
    return this.http.get(this.apiUrl + '/checkLogin')
  }

  registerUser(user: User) {
    return this.http.post<jwtToken>(this.apiUrl + '/register', user)
  }

  loginUser(user: User) {
    return this.http.post<jwtToken>(this.apiUrl + '/login', user)
  }

  changePassword(changePassword: ChangePassword) {
    return this.http.post<string>(this.apiUrl + '/changePassword', changePassword)
  }

  uploadProxies(formData: FormData) {
    return this.http.post<{proxyCount: number}>(this.apiUrl + '/addProxies', formData);
  }

  deleteProxies(settings: DeleteSettings) {
    return this.http.request<string>('delete', this.apiUrl + '/proxies', {
      body: settings,
    });
  }


  getProxyPage(pageNumber: number, options?: { rows?: number; search?: string }) {
    let params = new HttpParams();

    if (options?.rows && options.rows > 0) {
      params = params.set('pageSize', options.rows.toString());
    }

    if (options?.search && options.search.trim().length > 0) {
      params = params.set('search', options.search.trim());
    }

    return this.http.get<ProxyPage>(`${this.apiUrl}/getProxyPage/${pageNumber}`, { params });
  }

  getProxyCount() {
    return this.http.get<number>(this.apiUrl + '/getProxyCount');
  }

  getProxyDetail(proxyId: number) {
    return this.http.get<ProxyDetail>(`${this.apiUrl}/proxies/${proxyId}`);
  }

  getProxyStatistics(proxyId: number, options?: { limit?: number }) {
    let params = new HttpParams();
    if (options?.limit && options.limit > 0) {
      params = params.set('limit', options.limit.toString());
    }

    return this.http.get<{statistics: ProxyStatistic[]}>(`${this.apiUrl}/proxies/${proxyId}/statistics`, { params })
      .pipe(map(res => res?.statistics ?? []));
  }

  getProxyStatisticResponseBody(proxyId: number, statisticId: number) {
    return this.http
      .get<ProxyStatisticResponseDetail>(`${this.apiUrl}/proxies/${proxyId}/statistics/${statisticId}`)
      .pipe(
        map(res => {
          const regex = res?.regex?.trim();
          return {
            response_body: res?.response_body ?? '',
            regex: regex ? regex : null,
          } as ProxyStatisticResponseDetail;
        })
      );
  }

  getRotatingProxies() {
    return this.http
      .get<{rotating_proxies: RotatingProxy[]}>(`${this.apiUrl}/rotatingProxies`)
      .pipe(map(res => res?.rotating_proxies ?? []));
  }

  createRotatingProxy(payload: CreateRotatingProxy) {
    return this.http.post<RotatingProxy>(`${this.apiUrl}/rotatingProxies`, payload);
  }

  deleteRotatingProxy(id: number) {
    return this.http.delete<void>(`${this.apiUrl}/rotatingProxies/${id}`);
  }

  getNextRotatingProxy(id: number) {
    return this.http.post<RotatingProxyNext>(`${this.apiUrl}/rotatingProxies/${id}/next`, {});
  }


  saveGlobalSettings(payload: GlobalSettings) {
    return this.http.post(environment.apiUrl + "/saveSettings", payload)
  }

  getGlobalSettings() {
    return this.http.get<GlobalSettings>(this.apiUrl + '/global/settings');
  }

  getUserSettings() {
    return this.http.get<UserSettings>(this.apiUrl + '/user/settings');
  }

  saveUserSettings(payload: UserSettings) {
    return this.http.post(environment.apiUrl + "/user/settings", payload)
  }

  saveUserScrapingSites(payload: string[]) {
    return this.http.post(environment.apiUrl + "/user/scrapingSites", payload)
  }

  getUserRole() {
    return this.http.get<string>(this.apiUrl + '/user/role');
  }

  exportProxies(settings: ExportSettings) {
    return this.http.post<string>(this.apiUrl + '/user/export', settings)
  }

  uploadScrapeSources(formData: FormData) {
    return this.http.post<{sourceCount: number}>(this.apiUrl + '/scrapingSources', formData);
  }

  getScrapingSourcesCount() {
    return this.http.get<number>(this.apiUrl + '/getScrapingSourcesCount');
  }

  getScrapingSourcePage(pageNumber: number) {
    return this.http.get<ScrapeSourceInfo[]>(this.apiUrl + '/getScrapingSourcesPage/' + pageNumber);
  }

  deleteScrapingSource(proxies: number[]) {
    return this.http.request<string>('delete', this.apiUrl + '/scrapingSources', {
      body: proxies,
    });
  }

  getDashboardInfo() {
    return this.http.get<DashboardInfo>(this.apiUrl + '/getDashboardInfo');
  }
}
