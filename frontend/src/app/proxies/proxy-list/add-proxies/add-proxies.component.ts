import {Component, EventEmitter, Output, computed, signal} from '@angular/core';
import {CommonModule} from '@angular/common';
import {FormsModule} from "@angular/forms";
import {ProcesingPopupComponent} from './procesing-popup/procesing-popup.component';
import {Button} from 'primeng/button';
import {DialogModule} from 'primeng/dialog';
import {TooltipComponent} from '../../../tooltip/tooltip.component';
import {HttpService} from '../../../services/http.service';
import {NotificationService} from '../../../services/notification-service.service';
import {AddProxiesDetails} from '../../../models/AddProxiesResponse';

@Component({
    selector: 'app-add-proxies',
  imports: [
    CommonModule,
    FormsModule,
    TooltipComponent,
    ProcesingPopupComponent,
    Button,
    DialogModule,
  ],
    templateUrl: './add-proxies.component.html',
    styleUrl: './add-proxies.component.scss'
})
export class AddProxiesComponent {
  @Output() showAddProxiesMessage = new EventEmitter<boolean>();
  @Output() proxiesAdded = new EventEmitter<void>();

  readonly file = signal<File | undefined>(undefined);
  readonly proxyTextarea = signal<string>("");
  readonly clipboardProxies = signal<string>("");

  readonly fileProxiesNoAuthCount = signal(0);
  readonly fileProxiesWithAuthCount = signal(0);
  readonly uniqueFileProxiesCount = signal(0);

  readonly textAreaProxiesNoAuthCount = signal(0);
  readonly textAreaProxiesWithAuthCount = signal(0);
  readonly uniqueTextAreaProxiesCount = signal(0);

  readonly clipboardProxiesNoAuthCount = signal(0);
  readonly clipboardProxiesWithAuthCount = signal(0);
  readonly uniqueClipboardProxiesCount = signal(0);

  readonly dialogVisible = signal(false);
  readonly showPopup = signal(false);
  readonly popupStatus = signal<'processing' | 'success' | 'error'>('processing');
  readonly addedProxyCount = signal(0);
  readonly detailsVisible = signal(false);
  readonly uploadDetails = signal<AddProxiesDetails | null>(null);
  readonly hasUploadDetails = computed(() => this.uploadDetails() !== null);

  readonly proxiesWithoutAuthCount = computed(() =>
    this.textAreaProxiesNoAuthCount() + this.fileProxiesNoAuthCount() + this.clipboardProxiesNoAuthCount()
  );
  readonly proxiesWithAuthCount = computed(() =>
    this.textAreaProxiesWithAuthCount() + this.fileProxiesWithAuthCount() + this.clipboardProxiesWithAuthCount()
  );
  readonly uniqueProxiesCount = computed(() =>
    this.uniqueFileProxiesCount() + this.uniqueTextAreaProxiesCount() + this.uniqueClipboardProxiesCount()
  );

  constructor(private service: HttpService) { }

  async pasteFromClipboard(): Promise<void> {
    try {
      const text = await navigator.clipboard.readText();
      this.clipboardProxies.set(text);
      this.processClipboardProxies();
    } catch (err) {
      console.error('Failed to read clipboard:', err);
    }
  }

  clearClipboardProxies(): void {
    this.clipboardProxies.set("");
    this.clipboardProxiesNoAuthCount.set(0);
    this.clipboardProxiesWithAuthCount.set(0);
    this.uniqueClipboardProxiesCount.set(0);
  }

  processClipboardProxies() {
    const clipboard = this.clipboardProxies();
    if (!clipboard) {
      this.clearClipboardProxies();
      return;
    }

    const lines = clipboard.split(/\r?\n/);
    const proxies = lines.filter(line => (line.match(/:/g) || []).length === 1);

    this.clipboardProxiesNoAuthCount.set(proxies.length);
    this.clipboardProxiesWithAuthCount.set(lines.filter(line => (line.match(/:/g) || []).length === 3).length);
    this.uniqueClipboardProxiesCount.set(Array.from(new Set(proxies)).length);
  }

  triggerFileInput(fileInput: HTMLInputElement): void {
    fileInput.click();
  }

  openDialog(): void {
    this.dialogVisible.set(true);
  }

  closeDialog(): void {
    this.dialogVisible.set(false);
    this.resetFormState();
  }

  onDialogHide(): void {
    this.resetFormState();
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      const file = input.files[0];
      this.file.set(file);

      const reader = new FileReader();
      reader.onload = (_: ProgressEvent<FileReader>) => {
        const content = reader.result as string;
        const lines = content.split(/\r?\n/);
        let proxies = lines.filter(line => (line.match(/:/g) || []).length === 1)

        this.fileProxiesNoAuthCount.set(proxies.length);
        this.fileProxiesWithAuthCount.set(lines.filter(line => (line.match(/:/g) || []).length === 3).length);
        this.uniqueFileProxiesCount.set(Array.from(new Set(proxies)).length);
      };

      reader.readAsText(file);
    }
  }

  onFileClear(): void {
    this.file.set(undefined);
    this.fileProxiesWithAuthCount.set(0);
    this.fileProxiesNoAuthCount.set(0);
    this.uniqueFileProxiesCount.set(0);
  }

  addTextAreaProxies() {
    const lines = this.proxyTextarea().split(/\r?\n/);
    let proxies = lines.filter(line => (line.match(/:/g) || []).length === 1)

    this.textAreaProxiesNoAuthCount.set(proxies.length);
    this.textAreaProxiesWithAuthCount.set(lines.filter(line => (line.match(/:/g) || []).length === 3).length);
    this.uniqueTextAreaProxiesCount.set(Array.from(new Set(proxies)).length);
  }

  onTextareaChange(value: string) {
    this.proxyTextarea.set(value);
    this.addTextAreaProxies();
  }

  getProxiesWithoutAuthCount() {
    return this.proxiesWithoutAuthCount();
  }

  getProxiesWithAuthCount() {
    return this.proxiesWithAuthCount();
  }

  getUniqueProxiesCount() {
    return this.uniqueProxiesCount();
  }

  submitProxies() {
    if (this.file() || this.proxyTextarea() || this.clipboardProxies()) {
      this.uploadDetails.set(null);
      this.detailsVisible.set(false);
      this.showPopup.set(true);
      this.popupStatus.set('processing');

      const formData = new FormData();

      const file = this.file();
      if (file) {
        formData.append('file', file);
      } else {
        formData.append('file', '');
      }

      if (this.proxyTextarea()) {
        formData.append('proxyTextarea', this.proxyTextarea());
      }

      if (this.clipboardProxies()) {
        formData.append('clipboardProxies', this.clipboardProxies());
      }

      this.service.uploadProxies(formData).subscribe({
        next: (response) => {
          this.uploadDetails.set(response.details ?? null);
          this.addedProxyCount.set(response.proxyCount);
          this.popupStatus.set('success');
          this.dialogVisible.set(false);

          this.resetFormState();
          this.showAddProxiesMessage.emit(false);
          this.proxiesAdded.emit();
        },
        error: (err) => {
          this.popupStatus.set('error');
          NotificationService.showError("Could not upload proxies: " + err.error.message)
        },
      });
    } else {
      console.warn('No data to submit');
    }
  }

  onPopupClose() {
    this.showPopup.set(false);
    this.uploadDetails.set(null);
    this.detailsVisible.set(false);
  }

  openDetails() {
    if (this.uploadDetails()) {
      this.detailsVisible.set(true);
    }
  }

  formatDuration(durationMs?: number | null): string {
    if (durationMs === null || durationMs === undefined) {
      return '-';
    }
    if (durationMs < 1000) {
      return `${durationMs} ms`;
    }
    return `${(durationMs / 1000).toFixed(2)} s`;
  }

  private resetFormState(): void {
    this.proxyTextarea.set("");
    this.clipboardProxies.set("");
    this.file.set(undefined);
    this.onFileClear();
    this.clearClipboardProxies();
    this.addTextAreaProxies();
  }
}
