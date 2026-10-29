export type ClipItem = { kind: "folder" | "file"; id: string };

const KEY = "localcloud.explorer.clipboard.v1";

type Clip = { mode: "cut" | "copy"; items: ClipItem[] };

function readClip(): Clip | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return null;
    const data = JSON.parse(raw) as Clip;
    if (data.mode !== "cut" && data.mode !== "copy") return null;
    if (!Array.isArray(data.items)) return null;
    return data;
  } catch {
    return null;
  }
}

function writeClip(mode: "cut" | "copy", items: ClipItem[]) {
  if (items.length === 0) {
    sessionStorage.removeItem(KEY);
    return;
  }
  sessionStorage.setItem(KEY, JSON.stringify({ mode, items } satisfies Clip));
}

export function setCutItems(items: ClipItem[]) {
  writeClip("cut", items);
}

export function setCopyItems(items: ClipItem[]) {
  writeClip("copy", items);
}

export function getClipboard(): Clip | null {
  return readClip();
}

export function getCutItems(): ClipItem[] {
  const clip = readClip();
  return clip?.mode === "cut" ? clip.items : [];
}

export function clearClipboard() {
  sessionStorage.removeItem(KEY);
}

export function clearCutItems() {
  const clip = readClip();
  if (clip?.mode === "cut") sessionStorage.removeItem(KEY);
}

export function hasClipboardItems() {
  return (readClip()?.items.length ?? 0) > 0;
}

export function hasCutItems() {
  return getCutItems().length > 0;
}
