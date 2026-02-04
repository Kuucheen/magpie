import {Component, signal} from '@angular/core';

import {ProxyListComponent} from './proxy-list/proxy-list.component';

@Component({
    selector: 'app-proxies',
  imports: [
    ProxyListComponent
],
    templateUrl: './proxies.component.html',
    styleUrl: './proxies.component.scss'
})
export class ProxiesComponent {
  showNoProxiesMessage = signal(false);
}
