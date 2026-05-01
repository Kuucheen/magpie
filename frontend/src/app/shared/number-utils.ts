export function normalizeNumber(value: number | string | null | undefined): number {
  if (value === null || value === undefined) {
    return 0;
  }
  const parsed = typeof value === 'string' ? Number(value) : value;
  if (!Number.isFinite(parsed)) {
    return 0;
  }
  return Math.max(0, Math.floor(parsed));
}
