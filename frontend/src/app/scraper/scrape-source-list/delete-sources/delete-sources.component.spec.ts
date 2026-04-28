import {ComponentFixture, TestBed} from '@angular/core/testing';
import {MessageService} from 'primeng/api';
import {of} from 'rxjs';
import {DeleteSourcesComponent} from './delete-sources.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';

describe('DeleteSourcesComponent', () => {
  let component: DeleteSourcesComponent;
  let fixture: ComponentFixture<DeleteSourcesComponent>;
  let deleteScrapingSourceSpy: jasmine.Spy;

  beforeEach(async () => {
    deleteScrapingSourceSpy = jasmine.createSpy('deleteScrapingSource').and.returnValue(of('Deleted 1 scraping sources.'));

    await TestBed.configureTestingModule({
      imports: [DeleteSourcesComponent],
      providers: [
        {provide: HttpService, useValue: {deleteScrapingSource: deleteScrapingSourceSpy}},
        {provide: NotificationService, useValue: {showError: jasmine.createSpy('showError'), showInfo: jasmine.createSpy('showInfo'), showSuccess: jasmine.createSpy('showSuccess')}},
        MessageService,
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(DeleteSourcesComponent);
    component = fixture.componentInstance;
    component.allSources = [{id: 1} as never];
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('deletes all sources with count filters', () => {
    component.deleteForm.patchValue({
      filter: true,
      proxyCountOperator: '<',
      proxyCount: 25,
      aliveCountOperator: '>',
      aliveCount: 10,
    });

    component.submitDelete();

    expect(deleteScrapingSourceSpy).toHaveBeenCalledWith(jasmine.objectContaining({
      scope: 'all',
      filter: true,
      proxyCountOperator: '<',
      proxyCount: 25,
      aliveCountOperator: '>',
      aliveCount: 10,
    }));
  });
});
