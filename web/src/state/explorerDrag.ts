export type ExplorerSel = { kind: "folder"; id: string } | { kind: "file"; id: string };

export const EXPLORER_DRAG_MIME = "application/x-localcloud-explorer-items";

export function isExplorerDrag(types: readonly string[]) {
  return types.includes(EXPLORER_DRAG_MIME);
}

export function writeExplorerDrag(dataTransfer: DataTransfer, items: ExplorerSel[]) {
  dataTransfer.setData(EXPLORER_DRAG_MIME, JSON.stringify(items));
  dataTransfer.effectAllowed = "move";
}

export function readExplorerDrag(dataTransfer: DataTransfer): ExplorerSel[] {
  const raw = dataTransfer.getData(EXPLORER_DRAG_MIME);
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as ExplorerSel[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((s) => s && (s.kind === "file" || s.kind === "folder") && typeof s.id === "string");
  } catch {
    return [];
  }
}
