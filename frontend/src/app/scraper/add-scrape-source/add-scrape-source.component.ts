import {Component, EventEmitter, Output, computed, signal} from '@angular/core';

import {FormsModule, ReactiveFormsModule} from "@angular/forms";
import {HttpService} from '../../services/http.service';

import {ButtonModule} from 'primeng/button';
import {TextareaModule} from 'primeng/textarea';
import {TooltipModule} from 'primeng/tooltip';
import {DialogModule} from 'primeng/dialog';
import {ClipboardService} from '../../services/clipboard.service';
import {NotificationService} from '../../services/notification-service.service';
import {
  ProcesingPopupComponent
} from '../../proxies/proxy-list/add-proxies/procesing-popup/procesing-popup.component';

@Component({
  selector: 'app-add-scrape-source',
  imports: [
    ProcesingPopupComponent,
    ReactiveFormsModule,
    FormsModule,
    ButtonModule,
    TextareaModule,
    TooltipModule,
    DialogModule
],
  templateUrl: './add-scrape-source.component.html',
  styleUrl: './add-scrape-source.component.scss'
})
export class AddScrapeSourceComponent {
  @Output() showAddScrapeSourcesMessage = new EventEmitter<boolean>();
  @Output() scrapeSourcesAdded = new EventEmitter<void>();

  readonly file = signal<File | undefined>(undefined);
  readonly scrapeSourceTextarea = signal<string>("");
  readonly clipboardScrapeSources = signal<string>("");

  readonly fileSourcesCount = signal(0);
  readonly uniqueFileSourcesCount = signal(0);

  readonly textAreaSourcesCount = signal(0);
  readonly uniqueTextAreaSourcesCount = signal(0);

  readonly clipboardSourcesCount = signal(0);
  readonly uniqueClipboardSourcesCount = signal(0);

  readonly dialogVisible = signal(false);
  readonly showPopup = signal(false);
  readonly popupStatus = signal<'processing' | 'success' | 'error'>('processing');
  readonly addedSourceCount = signal(0);

  readonly sourcesCount = computed(() =>
    this.textAreaSourcesCount() + this.fileSourcesCount() + this.clipboardSourcesCount()
  );
  readonly uniqueSourcesCount = computed(() =>
    this.uniqueFileSourcesCount() + this.uniqueTextAreaSourcesCount() + this.uniqueClipboardSourcesCount()
  );

  constructor(
    private service: HttpService,
    private clipboardService: ClipboardService,
    private notification: NotificationService
  ) { }

  async pasteFromClipboard(): Promise<void> {
    const text = await this.clipboardService.readText();
    if (text === null) {
      this.notification.showWarn('Could not read clipboard.');
      return;
    }
    this.clipboardScrapeSources.set(text);
    this.processClipboardSources();
  }

  clearClipboardSources(): void {
    this.clipboardScrapeSources.set("");
    this.clipboardSourcesCount.set(0);
    this.uniqueClipboardSourcesCount.set(0);
  }

  processClipboardSources() {
    const clipboard = this.clipboardScrapeSources();
    if (!clipboard) {
      this.clearClipboardSources();
      return;
    }

    const lines = clipboard.split(/\r?\n/);
    const sources = lines.filter(line => (line.match(/:/g) || []).length === 1);

    this.clipboardSourcesCount.set(sources.length);
    this.uniqueClipboardSourcesCount.set(Array.from(new Set(sources)).length);
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
        let sources = lines.filter(line => (line.match(/:/g) || []).length === 1)

        this.fileSourcesCount.set(sources.length);
        this.uniqueFileSourcesCount.set(Array.from(new Set(sources)).length);
      };

      reader.readAsText(file);
    }
  }

  onFileClear(): void {
    this.file.set(undefined);
    this.fileSourcesCount.set(0);
    this.uniqueFileSourcesCount.set(0);
  }

  addTextAreaSources() {
    const lines = this.scrapeSourceTextarea().split(/\r?\n/);
    let sources = lines.filter(line => (line.match(/:/g) || []).length === 1)

    this.textAreaSourcesCount.set(sources.length);
    this.uniqueTextAreaSourcesCount.set(Array.from(new Set(sources)).length);
  }

  onTextareaChange(value: string) {
    this.scrapeSourceTextarea.set(value);
    this.addTextAreaSources();
  }

  getSourcesCount() {
    return this.sourcesCount();
  }

  getUniqueSourcesCount() {
    return this.uniqueSourcesCount();
  }

  submitScrapeSources() {
    if (this.file() || this.scrapeSourceTextarea() || this.clipboardScrapeSources()) {
      this.showPopup.set(true);
      this.popupStatus.set('processing');

      const formData = new FormData();

      const file = this.file();
      if (file) {
        formData.append('file', file);
      } else {
        formData.append('file', '');
      }

      if (this.scrapeSourceTextarea()) {
        formData.append('scrapeSourceTextarea', this.scrapeSourceTextarea());
      }

      if (this.clipboardScrapeSources()) {
        formData.append('clipboardScrapeSources', this.clipboardScrapeSources());
      }

      this.service.uploadScrapeSources(formData).subscribe({
        next: (response) => {
          this.addedSourceCount.set(response.sourceCount);
          this.popupStatus.set('success');
          this.dialogVisible.set(false);

          this.showAddScrapeSourcesMessage.emit(false);
          this.scrapeSourcesAdded.emit();
          this.resetFormState();
        },
        error: (err) => {
          this.popupStatus.set('error');
          const reason = err?.error?.message ?? err?.error?.error ?? 'Unknown error';
          this.notification.showError("There has been an error while uploading the scrape sources! " + reason)
        },
      });
    } else {
      console.warn('No data to submit');
    }
  }

  onPopupClose() {
    this.showPopup.set(false);
  }

  private resetFormState(): void {
    this.scrapeSourceTextarea.set("");
    this.addTextAreaSources();
    this.clearClipboardSources();
    this.onFileClear();
  }
}
