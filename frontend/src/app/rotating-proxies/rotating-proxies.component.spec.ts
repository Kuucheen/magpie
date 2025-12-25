import {ComponentFixture, TestBed} from '@angular/core/testing';
import {of} from 'rxjs';
import {MessageService} from 'primeng/api';

import {RotatingProxiesComponent} from './rotating-proxies.component';
import {HttpService} from '../services/http.service';
import {NotificationService} from '../services/notification-service.service';

const httpServiceMock = {
  getRotatingProxies: jasmine.createSpy().and.returnValue(of([])),
  getUserSettings: jasmine.createSpy().and.returnValue(of({
    http_protocol: true,
    https_protocol: true,
    socks4_protocol: false,
    socks5_protocol: false,
    timeout: 5000,
    retries: 2,
    UseHttpsForSocks: true,
    transport_protocol: 'tcp',
    auto_remove_failing_proxies: false,
    auto_remove_failure_threshold: 3,
    judges: [],
    scraping_sources: [],
  })),
  createRotatingProxy: jasmine.createSpy().and.returnValue(of({
    id: 1,
    name: 'Test rotator',
    protocol: 'http',
    listen_protocol: 'http',
    alive_proxy_count: 0,
    listen_port: 19000,
    auth_required: false,
    reputation_labels: ['good', 'neutral'],
    created_at: new Date().toISOString(),
  })),
  deleteRotatingProxy: jasmine.createSpy().and.returnValue(of(void 0)),
  getNextRotatingProxy: jasmine.createSpy().and.returnValue(of({
    proxy_id: 1,
    ip: '127.0.0.1',
    port: 8000,
    has_auth: false,
    protocol: 'http',
  })),
};

describe('RotatingProxiesComponent', () => {
  let component: RotatingProxiesComponent;
  let fixture: ComponentFixture<RotatingProxiesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [RotatingProxiesComponent],
      providers: [
        MessageService,
        NotificationService,
        {provide: HttpService, useValue: httpServiceMock},
      ]
    })
    .compileComponents();

    TestBed.inject(NotificationService);

    fixture = TestBed.createComponent(RotatingProxiesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
