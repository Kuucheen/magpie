import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ProcesingPopupComponent } from './procesing-popup.component';
import {provideNoopAnimations} from '@angular/platform-browser/animations';

describe('ProcesingPopupComponent', () => {
  let component: ProcesingPopupComponent;
  let fixture: ComponentFixture<ProcesingPopupComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ProcesingPopupComponent],
      providers: [provideNoopAnimations()]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ProcesingPopupComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
