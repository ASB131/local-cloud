export type TransferKind = "upload" | "download";

export type TransferStatus = "queued" | "running" | "done" | "error" | "cancelled" | "conflict";

export type ConflictDetails = {
  file_id: string;
  size_bytes?: number;
  updated_at?: string;
};

export type TransferItem = {
  id: string;
  kind: TransferKind;
  name: string;
  folderId?: string;
  totalBytes: number;
  doneBytes: number;
  status: TransferStatus;
  error?: string;
  sessionId?: string;
  conflict?: ConflictDetails;
};

export type TransferAction =
  | { type: "add"; item: TransferItem }
  | { type: "patch"; id: string; patch: Partial<TransferItem> }
  | { type: "remove"; id: string }
  | { type: "cancel"; id: string };

export function transferReducer(state: TransferItem[], action: TransferAction): TransferItem[] {
  switch (action.type) {
    case "add":
      return [...state, action.item];
    case "patch":
      return state.map((t) => (t.id === action.id ? { ...t, ...action.patch } : t));
    case "remove":
      return state.filter((t) => t.id !== action.id);
    case "cancel":
      return state.map((t) =>
        t.id === action.id && (t.status === "queued" || t.status === "running")
          ? { ...t, status: "cancelled" }
          : t,
      );
    default:
      return state;
  }
}

const STORAGE_KEY = "localcloud.transfers.v1";

export function loadPersistedTransfers(): TransferItem[] {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as TransferItem[];
    return parsed.filter((t) => t.status === "running" || t.status === "queued");
  } catch {
    return [];
  }
}

export function persistTransfers(items: TransferItem[]) {
  const active = items.filter((t) => t.status === "running" || t.status === "queued");
  sessionStorage.setItem(STORAGE_KEY, JSON.stringify(active));
}
