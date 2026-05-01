export function buildDatedExportFileName(suffix: string): string {
  const today = new Date();
  const year = today.getFullYear();
  const month = String(today.getMonth() + 1).padStart(2, '0');
  const day = String(today.getDate()).padStart(2, '0');
  const randomCode = generateRandomCode(4);
  return `${year}-${month}-${day}-${randomCode}-${suffix}`;
}

export function downloadTextFile(data: BlobPart, fileName: string): void {
  const blob = new Blob([data], {type: 'text/plain'});
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  window.URL.revokeObjectURL(url);
}

export function extractHttpErrorMessage(error: unknown): string {
  if (typeof error !== 'object' || error === null) {
    return 'Unknown error';
  }

  const httpError = error as {error?: unknown; message?: unknown};
  const nestedMessage = extractPayloadMessage(httpError.error);

  if (nestedMessage) {
    return nestedMessage;
  }

  if (typeof httpError.message === 'string' && httpError.message.trim().length > 0) {
    return httpError.message;
  }

  return 'Unknown error';
}

function generateRandomCode(length: number): string {
  const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += characters.charAt(Math.floor(Math.random() * characters.length));
  }
  return result;
}

function extractPayloadMessage(payload: unknown): string | null {
  if (typeof payload === 'string' && payload.trim().length > 0) {
    try {
      const parsed = JSON.parse(payload) as {error?: unknown; message?: unknown};
      return extractPayloadMessage(parsed) ?? payload;
    } catch {
      return payload;
    }
  }

  if (typeof payload === 'object' && payload !== null) {
    const body = payload as {error?: unknown; message?: unknown};
    if (typeof body.error === 'string' && body.error.trim().length > 0) {
      return body.error;
    }
    if (typeof body.message === 'string' && body.message.trim().length > 0) {
      return body.message;
    }
  }

  return null;
}
