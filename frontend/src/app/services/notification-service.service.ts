// src/app/services/notification.service.ts
import { Injectable } from '@angular/core';
import { MessageService } from 'primeng/api';

@Injectable({ providedIn: 'root' })
export class NotificationService {
  constructor(private messageService: MessageService) {}

  showError(detail: string, summary = 'Error') {
    this.messageService.add({ severity: 'error', summary, detail, life: 6000 });
  }
  showSuccess(detail: string, summary = 'Success') {
    this.messageService.add({ severity: 'success', summary, detail, life: 4000 });
  }
  showInfo(detail: string, summary = 'Info') {
    this.messageService.add({ severity: 'info', summary, detail, life: 4000 });
  }
  showWarn(detail: string, summary = 'Warning') {
    this.messageService.add({ severity: 'warn', summary, detail, life: 5000 });
  }
}
