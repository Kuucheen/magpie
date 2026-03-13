import { ComponentFixture, TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AdminScraperComponent } from './admin-scraper.component';
import { SettingsService } from '../../services/settings.service';
import { NotificationService } from '../../services/notification-service.service';

class SettingsServiceStub {
  settings$ = of({
    scraper: {
      dynamic_threads: true,
      threads: 250,
      max_threads: 500,
      retries: 2,
      timeout: 7500,
      respect_robots_txt: true,
      scrape_sites: ['https://example.com/proxies.txt'],
      scraper_timer: { days: 0, hours: 9, minutes: 0, seconds: 0 }
    },
    proxy_limits: {
      enabled: false,
      max_per_user: 0,
      exclude_admins: true
    }
  });

  saveGlobalSettings() {
    return of({ message: 'saved' });
  }

  requeueAllScrapeSources() {
    return of({ message: 'requeued', source_count: 0 });
  }
}

class NotificationServiceStub {
  showError = jasmine.createSpy('showError');
  showSuccess = jasmine.createSpy('showSuccess');
}

describe('AdminScraperComponent', () => {
  let component: AdminScraperComponent;
  let fixture: ComponentFixture<AdminScraperComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AdminScraperComponent],
      providers: [
        { provide: SettingsService, useClass: SettingsServiceStub },
        { provide: NotificationService, useClass: NotificationServiceStub }
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(AdminScraperComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
