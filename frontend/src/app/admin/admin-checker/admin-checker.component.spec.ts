import { ComponentFixture, TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AdminCheckerComponent } from './admin-checker.component';
import { SettingsService } from '../../services/settings.service';
import { NotificationService } from '../../services/notification-service.service';

class SettingsServiceStub {
  settings$ = of({
    protocols: { http: false, https: true, socks4: false, socks5: false },
    blacklist_sources: []
  });

  getCheckerSettings() {
    return of({
      dynamic_threads: false,
      threads: 250,
      max_threads: 250,
      save_responses: true,
      retries: 2,
      timeout: 7500,
      checker_timer: { days: 0, hours: 1, minutes: 0, seconds: 0 },
      judge_timer: { days: 0, hours: 0, minutes: 30, seconds: 0 },
      ip_lookup: 'https://ident.me',
      judges_threads: 3,
      judges_timeout: 5000,
      use_https_for_socks: true,
      judges: [
        { url: 'https://pool.proxyspace.pro/judge.php', regex: 'default' },
        { url: 'http://azenv.net', regex: 'default' }
      ],
      standard_header: ['USER-AGENT', 'HOST'],
      proxy_header: ['HTTP_X_FORWARDED_FOR']
    });
  }

  getGlobalSettings() {
    return {
      protocols: { http: false, https: true, socks4: false, socks5: false },
      blacklist_sources: []
    };
  }

  saveGlobalSettings() {
    return of({ message: 'saved' });
  }

  requeueAllProxies() {
    return of({ message: 'requeued', proxy_count: 0 });
  }
}

class NotificationServiceStub {
  showError = jasmine.createSpy('showError');
  showSuccess = jasmine.createSpy('showSuccess');
}

describe('AdminCheckerComponent', () => {
  let component: AdminCheckerComponent;
  let fixture: ComponentFixture<AdminCheckerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AdminCheckerComponent],
      providers: [
        { provide: SettingsService, useClass: SettingsServiceStub },
        { provide: NotificationService, useClass: NotificationServiceStub }
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(AdminCheckerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
