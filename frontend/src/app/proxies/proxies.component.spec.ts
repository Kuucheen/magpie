import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';
import { ProxiesComponent } from './proxies.component';

describe('ProxiesComponent', () => {
  let component: ProxiesComponent;
  let fixture: ComponentFixture<ProxiesComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ProxiesComponent],
      providers: [MessageService]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ProxiesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
