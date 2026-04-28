import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {of, throwError} from 'rxjs';
import {ExportSourcesComponent} from './export-sources.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';

describe('ExportSourcesComponent', () => {
  let component: ExportSourcesComponent;
  let fixture: ComponentFixture<ExportSourcesComponent>;
  let exportScrapeSourcesSpy: jasmine.Spy;
  let showErrorSpy: jasmine.Spy;
  let downloadFileSpy: jasmine.Spy;

  beforeEach(async () => {
    exportScrapeSourcesSpy = jasmine.createSpy('exportScrapeSources');
    showErrorSpy = jasmine.createSpy('showError');

    await TestBed.configureTestingModule({
      imports: [ExportSourcesComponent],
      providers: [
        {provide: HttpService, useValue: {exportScrapeSources: exportScrapeSourcesSpy}},
        {provide: NotificationService, useValue: {showError: showErrorSpy}},
        MessageService,
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(ExportSourcesComponent);
    component = fixture.componentInstance;
    component.allSources = [{id: 1} as never];
    downloadFileSpy = spyOn<any>(component, 'downloadFile');
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('downloads plain text export responses', () => {
    exportScrapeSourcesSpy.and.returnValue(of('https://example.com;10;5'));

    component.submitExport();

    expect(exportScrapeSourcesSpy).toHaveBeenCalled();
    expect(downloadFileSpy).toHaveBeenCalledWith('https://example.com;10;5', jasmine.any(String));
  });

  it('uses protocol url as the default output format', () => {
    exportScrapeSourcesSpy.and.returnValue(of('https://example.com'));

    component.submitExport();

    expect(exportScrapeSourcesSpy).toHaveBeenCalledWith(jasmine.objectContaining({
      outputFormat: 'protocol://url',
    }));
  });

  it('sends less-than count filters', () => {
    exportScrapeSourcesSpy.and.returnValue(of('https://example.com'));
    component.exportForm.patchValue({
      filter: true,
      proxyCountOperator: '<',
      proxyCount: 25,
      aliveCountOperator: '<',
      aliveCount: 10,
    });

    component.submitExport();

    expect(exportScrapeSourcesSpy).toHaveBeenCalledWith(jasmine.objectContaining({
      filter: true,
      proxyCountOperator: '<',
      proxyCount: 25,
      aliveCountOperator: '<',
      aliveCount: 10,
    }));
  });

  it('shows backend JSON error messages returned as text', () => {
    exportScrapeSourcesSpy.and.returnValue(
      throwError(() => ({
        error: '{"error":"Could not export scrape sources"}',
        message: 'Http failure response for /api/scrapingSources/export: 500 Internal Server Error',
      }))
    );

    component.submitExport();

    expect(showErrorSpy).toHaveBeenCalledWith('Error while exporting scrape sources: Could not export scrape sources');
  });
});
