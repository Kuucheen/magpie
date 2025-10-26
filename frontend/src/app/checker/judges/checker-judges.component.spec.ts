import {ComponentFixture, TestBed} from '@angular/core/testing';
import {of} from 'rxjs';
import {CheckerJudgesComponent} from './checker-judges.component';
import {UserSettings} from '../../models/UserSettings';
import {SettingsService} from '../../services/settings.service';

class SettingsServiceStub {
  private settings: UserSettings = {
    http_protocol: true,
    https_protocol: true,
    socks4_protocol: false,
    socks5_protocol: false,
    timeout: 7500,
    retries: 2,
    UseHttpsForSocks: true,
    auto_remove_failing_proxies: false,
    auto_remove_failure_threshold: 3,
    judges: [{ url: 'https://example.com', regex: 'default' }],
    scraping_sources: []
  };

  getUserSettings(): UserSettings {
    return this.settings;
  }

  saveUserSettings(_: any) {
    return of({ message: 'saved' });
  }
}

describe('CheckerJudgesComponent', () => {
  let component: CheckerJudgesComponent;
  let fixture: ComponentFixture<CheckerJudgesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CheckerJudgesComponent],
      providers: [{ provide: SettingsService, useClass: SettingsServiceStub }]
    }).compileComponents();

    fixture = TestBed.createComponent(CheckerJudgesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
    expect(component.judgeControls.length).toBeGreaterThan(0);
  });
});
