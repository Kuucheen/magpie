import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {RouterTestingModule} from '@angular/router/testing';
import {of} from 'rxjs';
import {ScrapeSourceListComponent} from './scrape-source-list.component';
import {HttpService} from '../../services/http.service';
import {SettingsService} from '../../services/settings.service';

describe('ScrapeSourceListComponent', () => {
  let component: ScrapeSourceListComponent;
  let fixture: ComponentFixture<ScrapeSourceListComponent>;

  beforeEach(async () => {
    const httpServiceStub = {
      getRespectRobotsSetting: jasmine.createSpy('getRespectRobotsSetting').and.returnValue(of({respect_robots_txt: false})),
      getScrapingSourcesCount: jasmine.createSpy('getScrapingSourcesCount').and.returnValue(of(0)),
      getScrapingSourcePage: jasmine.createSpy('getScrapingSourcePage').and.returnValue(of([])),
    } satisfies Partial<HttpService>;
    const settingsServiceStub = {
      getUserSettings: jasmine.createSpy('getUserSettings').and.returnValue(undefined),
      userSettings$: of(undefined),
      saveScrapeSourceListColumns: jasmine.createSpy('saveScrapeSourceListColumns').and.returnValue(of({message: 'ok'})),
    } satisfies Partial<SettingsService>;

    await TestBed.configureTestingModule({
      imports: [ScrapeSourceListComponent, RouterTestingModule],
      providers: [
        MessageService,
        {provide: HttpService, useValue: httpServiceStub},
        {provide: SettingsService, useValue: settingsServiceStub},
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ScrapeSourceListComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
