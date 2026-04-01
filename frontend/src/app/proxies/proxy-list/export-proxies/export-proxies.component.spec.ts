import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {of, throwError} from 'rxjs';
import {ExportProxiesComponent} from './export-proxies.component';
import {SettingsService} from '../../../services/settings.service';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';

describe('ExportProxiesComponent', () => {
  let component: ExportProxiesComponent;
  let fixture: ComponentFixture<ExportProxiesComponent>;
  let exportProxiesSpy: jasmine.Spy;
  let showErrorSpy: jasmine.Spy;
  let downloadFileSpy: jasmine.Spy;

  beforeEach(async () => {
    exportProxiesSpy = jasmine.createSpy('exportProxies');
    showErrorSpy = jasmine.createSpy('showError');

    await TestBed.configureTestingModule({
      imports: [ExportProxiesComponent],
      providers: [
        {provide: SettingsService, useValue: {getUserSettings: () => ({})}},
        {provide: HttpService, useValue: {exportProxies: exportProxiesSpy}},
        {provide: NotificationService, useValue: {showError: showErrorSpy}},
        MessageService,
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(ExportProxiesComponent);
    component = fixture.componentInstance;
    component.allProxies = [{id: 1} as never];
    downloadFileSpy = spyOn<any>(component, 'downloadFile');
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('downloads plain text export responses', () => {
    exportProxiesSpy.and.returnValue(of('http://127.0.0.1:8080'));

    component.submitExport();

    expect(exportProxiesSpy).toHaveBeenCalled();
    expect(downloadFileSpy).toHaveBeenCalledWith('http://127.0.0.1:8080', jasmine.any(String));
  });

  it('shows backend JSON error messages returned as text', () => {
    exportProxiesSpy.and.returnValue(
      throwError(() => ({
        error: '{"error":"Could not export proxies"}',
        message: 'Http failure response for /api/user/export: 500 Internal Server Error',
      }))
    );

    component.submitExport();

    expect(showErrorSpy).toHaveBeenCalledWith('Error while exporting proxies: Could not export proxies');
  });
});
