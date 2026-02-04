import {Component, inject} from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { NavbarComponent } from './navbar/navbar.component';
import { UserService } from './services/authorization/user.service';
import {Toast} from 'primeng/toast';
import {LayoutService} from './services/layout.service';
import {TopbarComponent} from './navbar/topbar/topbar.component';
import {LoadingComponent} from './ui-elements/loading/loading.component';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, NavbarComponent, Toast, TopbarComponent, LoadingComponent],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent {
  title = 'Magpie';
  layout = inject(LayoutService);
  private readonly userService = inject(UserService);

  protected readonly UserService = UserService;
}
