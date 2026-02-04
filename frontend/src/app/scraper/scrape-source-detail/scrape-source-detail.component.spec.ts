import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {ActivatedRoute, convertToParamMap} from '@angular/router';
import {RouterTestingModule} from '@angular/router/testing';
import {of} from 'rxjs';
import {ScrapeSourceDetailComponent} from './scrape-source-detail.component';
import {ScrapeSourceDetail} from '../../models/ScrapeSourceDetail';
import {HttpService} from '../../services/http.service';

describe('ScrapeSourceDetailComponent', () => {
  let component: ScrapeSourceDetailComponent;
  let fixture: ComponentFixture<ScrapeSourceDetailComponent>;

  beforeEach(async () => {
    const detail: ScrapeSourceDetail = {
      id: 1,
      url: 'https://example.com',
      added_at: new Date().toISOString(),
      proxy_count: 24,
      alive_count: 14,
      dead_count: 6,
      unknown_count: 4,
      avg_reputation: 68,
      last_proxy_added_at: new Date().toISOString(),
      last_checked_at: new Date().toISOString(),
      reputation_breakdown: {
        good: 8,
        neutral: 7,
        poor: 5,
        unknown: 4,
      }
    };

    const httpServiceStub = {
      getScrapeSourceDetail: jasmine.createSpy('getScrapeSourceDetail').and.returnValue(of(detail)),
      getScrapeSourceProxyPage: jasmine.createSpy('getScrapeSourceProxyPage').and.returnValue(of({ proxies: [], total: 0 })),
      getProxyFilterOptions: jasmine.createSpy('getProxyFilterOptions').and.returnValue(of({countries: [], types: [], anonymityLevels: []})),
    } satisfies Partial<HttpService>;

    await TestBed.configureTestingModule({
      imports: [ScrapeSourceDetailComponent, RouterTestingModule],
      providers: [
        MessageService,
        {provide: HttpService, useValue: httpServiceStub},
        {
          provide: ActivatedRoute,
          useValue: {
            paramMap: of(convertToParamMap({id: '1'})),
          }
        }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(ScrapeSourceDetailComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
