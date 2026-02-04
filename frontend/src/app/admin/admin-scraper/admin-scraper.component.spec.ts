import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';
import { AdminScraperComponent } from './admin-scraper.component';

describe('AdminScraperComponent', () => {
  let component: AdminScraperComponent;
  let fixture: ComponentFixture<AdminScraperComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AdminScraperComponent],
      providers: [MessageService]
    })
    .compileComponents();

    fixture = TestBed.createComponent(AdminScraperComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
