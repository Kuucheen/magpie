import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';
import { AdminBlacklistComponent } from './admin-blacklist.component';

describe('AdminBlacklistComponent', () => {
  let component: AdminBlacklistComponent;
  let fixture: ComponentFixture<AdminBlacklistComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AdminBlacklistComponent],
      providers: [MessageService]
    })
    .compileComponents();

    fixture = TestBed.createComponent(AdminBlacklistComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
