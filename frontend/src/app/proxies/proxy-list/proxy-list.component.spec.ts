import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';
import { ProxyListComponent } from './proxy-list.component';

describe('ProxyListComponent', () => {
  let component: ProxyListComponent;
  let fixture: ComponentFixture<ProxyListComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ProxyListComponent],
      providers: [MessageService]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ProxyListComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
