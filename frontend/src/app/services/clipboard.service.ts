import {Injectable} from '@angular/core';

@Injectable({
  providedIn: 'root',
})
export class ClipboardService {
  async copyText(value: string): Promise<boolean> {
    const text = `${value ?? ''}`;
    if (!text) {
      return false;
    }

    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch {
        // Fall back to execCommand if clipboard permissions are denied.
      }
    }

    if (typeof document === 'undefined' || !document.body) {
      return false;
    }

    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    textarea.setSelectionRange(0, textarea.value.length);

    try {
      const copied = document.execCommand('copy');
      document.body.removeChild(textarea);
      return copied;
    } catch {
      document.body.removeChild(textarea);
      return false;
    }
  }
}
