import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {of} from 'rxjs';
import {ExportProxiesComponent} from './export-proxies.component';
import {SettingsService} from '../../../services/settings.service';
import {HttpService} from '../../../services/http.service';

describe('ExportProxiesComponent', () => {
  let component: ExportProxiesComponent;
  let fixture: ComponentFixture<ExportProxiesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ExportProxiesComponent],
      providers: [
        {provide: SettingsService, useValue: {getUserSettings: () => ({})}},
        {provide: HttpService, useValue: {exportProxies: () => of('')}},
        MessageService,
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(ExportProxiesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
