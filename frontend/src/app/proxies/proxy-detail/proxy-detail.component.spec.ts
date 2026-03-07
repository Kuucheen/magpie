import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {ProxyDetailComponent} from './proxy-detail.component';
import {ActivatedRoute, convertToParamMap} from '@angular/router';
import {RouterTestingModule} from '@angular/router/testing';
import {of} from 'rxjs';
import {By} from '@angular/platform-browser';
import {HttpService} from '../../services/http.service';
import {ProxyDetail} from '../../models/ProxyDetail';
import {ProxyStatistic} from '../../models/ProxyStatistic';

describe('ProxyDetailComponent', () => {
  let component: ProxyDetailComponent;
  let fixture: ComponentFixture<ProxyDetailComponent>;

  beforeEach(async () => {
    const detail: ProxyDetail = {
      id: 1,
      ip: '127.0.0.1',
      port: 8080,
      username: '',
      password: '',
      has_auth: false,
      estimated_type: 'datacenter',
      country: 'Unknown',
      created_at: new Date().toISOString(),
      latest_check: new Date().toISOString(),
      latest_statistic: null,
    };

    const httpServiceStub = {
      getProxyDetail: jasmine.createSpy('getProxyDetail').and.returnValue(of(detail)),
      getProxyStatistics: jasmine.createSpy('getProxyStatistics').and.returnValue(of([] as ProxyStatistic[])),
    } satisfies Partial<HttpService>;

    await TestBed.configureTestingModule({
      imports: [ProxyDetailComponent, RouterTestingModule],
      providers: [
        MessageService,
        {provide: HttpService, useValue: httpServiceStub},
        {
          provide: ActivatedRoute,
          useValue: {
            paramMap: of(convertToParamMap({id: '1'})),
            snapshot: {
              queryParamMap: convertToParamMap({}),
            },
          }
        }
      ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ProxyDetailComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should truncate the latest judge and expose the full value in the title', () => {
    const longJudge = 'https://judge.example.com/some/really/long/path/that/should/not/wrap/in/the/card';
    const statistic: ProxyStatistic = {
      id: 10,
      attempt: 1,
      protocol: 'http',
      judge: longJudge,
      anonymity_level: 'elite',
      response_time: 250,
      alive: true,
      created_at: new Date().toISOString(),
    };

    component.detail.update(detail => detail ? {...detail, latest_statistic: statistic} : detail);
    fixture.detectChanges();

    const judgeValue = fixture.debugElement.queryAll(By.css('.detail-item .value--truncate'))
      .map(debugElement => debugElement.nativeElement as HTMLDivElement)
      .find(element => element.textContent?.includes(longJudge));

    expect(judgeValue).toBeDefined();
    expect(judgeValue?.title).toBe(longJudge);
  });
});
