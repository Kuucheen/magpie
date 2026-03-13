import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {BehaviorSubject, of} from 'rxjs';
import {RouterTestingModule} from '@angular/router/testing';
import {ProxyListComponent} from './proxy-list.component';
import {HttpService} from '../../services/http.service';
import {SettingsService} from '../../services/settings.service';
import {UserService} from '../../services/authorization/user.service';

describe('ProxyListComponent', () => {
  let component: ProxyListComponent;
  let fixture: ComponentFixture<ProxyListComponent>;

  beforeEach(async () => {
    const httpServiceStub = {
      getProxyPage: jasmine.createSpy('getProxyPage').and.returnValue(of({proxies: [], total: 0})),
      getProxyFilterOptions: jasmine.createSpy('getProxyFilterOptions').and.returnValue(of({countries: [], types: [], anonymityLevels: []})),
      requeueProxy: jasmine.createSpy('requeueProxy').and.returnValue(of({message: 'Proxy queued successfully', proxy_id: 1})),
    } satisfies Partial<HttpService>;
    const settingsServiceStub = {
      getUserSettings: jasmine.createSpy('getUserSettings').and.returnValue(undefined),
      userSettings$: of(undefined),
      saveProxyListColumns: jasmine.createSpy('saveProxyListColumns').and.returnValue(of({message: 'ok'})),
    } satisfies Partial<SettingsService>;
    const role$ = new BehaviorSubject<string | undefined>('user');
    const userServiceStub = {
      role$: role$.asObservable(),
    } satisfies Partial<UserService>;

    await TestBed.configureTestingModule({
      imports: [ProxyListComponent, RouterTestingModule],
      providers: [
        MessageService,
        {provide: HttpService, useValue: httpServiceStub},
        {provide: SettingsService, useValue: settingsServiceStub},
        {provide: UserService, useValue: userServiceStub},
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ProxyListComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
