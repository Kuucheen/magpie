import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';
import {BehaviorSubject, of} from 'rxjs';

import { AdminOtherComponent } from './admin-other.component';
import {SettingsService} from '../../services/settings.service';
import {GlobalSettings} from '../../models/GlobalSettings';

const timer = () => ({ days: 0, hours: 0, minutes: 0, seconds: 0 });

const defaultSettings: GlobalSettings = {
  protocols: { http: false, https: true, socks4: false, socks5: false },
  checker: {
    dynamic_threads: true,
    threads: 250,
    retries: 2,
    timeout: 7500,
    checker_timer: timer(),
    judges_threads: 1,
    judges_timeout: 1000,
    judges: [],
    judge_timer: timer(),
    use_https_for_socks: true,
    ip_lookup: '',
    standard_header: [],
    proxy_header: []
  },
  scraper: {
    dynamic_threads: true,
    threads: 1,
    retries: 1,
    timeout: 1000,
    respect_robots_txt: true,
    scraper_timer: timer(),
    scrape_sites: []
  },
  proxy_limits: {
    enabled: false,
    max_per_user: 0,
    exclude_admins: true
  },
  geolite: {
    api_key: '',
    auto_update: false,
    update_timer: timer(),
    last_updated_at: null
  },
  blacklist_sources: [],
  website_blacklist: []
};

class SettingsServiceStub {
  settings$ = new BehaviorSubject<GlobalSettings | undefined>(defaultSettings);

  saveGlobalSettings = jasmine.createSpy('saveGlobalSettings').and.returnValue(of({ message: 'Saved' }));
}

describe('AdminOtherComponent', () => {
  let component: AdminOtherComponent;
  let fixture: ComponentFixture<AdminOtherComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AdminOtherComponent],
      providers: [
        { provide: SettingsService, useClass: SettingsServiceStub },
        MessageService,
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(AdminOtherComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
