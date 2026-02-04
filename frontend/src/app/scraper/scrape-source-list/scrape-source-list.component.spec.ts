import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {RouterTestingModule} from '@angular/router/testing';
import {of} from 'rxjs';
import {ScrapeSourceListComponent} from './scrape-source-list.component';
import {HttpService} from '../../services/http.service';

describe('ScrapeSourceListComponent', () => {
  let component: ScrapeSourceListComponent;
  let fixture: ComponentFixture<ScrapeSourceListComponent>;

  beforeEach(async () => {
    const httpServiceStub = {
      getRespectRobotsSetting: jasmine.createSpy('getRespectRobotsSetting').and.returnValue(of({respect_robots_txt: false})),
      getScrapingSourcesCount: jasmine.createSpy('getScrapingSourcesCount').and.returnValue(of(0)),
      getScrapingSourcePage: jasmine.createSpy('getScrapingSourcePage').and.returnValue(of([])),
    } satisfies Partial<HttpService>;

    await TestBed.configureTestingModule({
      imports: [ScrapeSourceListComponent, RouterTestingModule],
      providers: [
        MessageService,
        {provide: HttpService, useValue: httpServiceStub},
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
