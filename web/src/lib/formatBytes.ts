const GIB = 1024 ** 3;

export function bytesToGiB(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0";
  const gib = bytes / GIB;
  return gib >= 10 ? gib.toFixed(0) : gib.toFixed(2);
}

export function gibToBytes(gib: number): number {
  if (!Number.isFinite(gib) || gib < 0) return 0;
  return Math.round(gib * GIB);
}

export function formatUsedQuota(used: number, quota: number): string {
  return `${bytesToGiB(used)} / ${bytesToGiB(quota)} GiB`;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString();
}
