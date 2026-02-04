import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';
import { AddScrapeSourceComponent } from './add-scrape-source.component';

describe('AddScrapeSourceComponent', () => {
  let component: AddScrapeSourceComponent;
  let fixture: ComponentFixture<AddScrapeSourceComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AddScrapeSourceComponent],
      providers: [MessageService]
    })
    .compileComponents();

    fixture = TestBed.createComponent(AddScrapeSourceComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
