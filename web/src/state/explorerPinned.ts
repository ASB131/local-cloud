const KEY = "localcloud.pinnedFolders.v1";
const MAX = 8;

export type PinnedFolder = {
  id: string;
  name: string;
  pinnedAt: number;
};

export function loadPinnedFolders(): PinnedFolder[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as PinnedFolder[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((x) => x && typeof x.id === "string").slice(0, MAX);
  } catch {
    return [];
  }
}

export function savePinnedFolders(list: PinnedFolder[]) {
  localStorage.setItem(KEY, JSON.stringify(list.slice(0, MAX)));
}

export function pinFolder(entry: { id: string; name: string }) {
  const list = loadPinnedFolders().filter((x) => x.id !== entry.id);
  list.unshift({ id: entry.id, name: entry.name, pinnedAt: Date.now() });
  savePinnedFolders(list);
}

export function unpinFolder(folderId: string) {
  savePinnedFolders(loadPinnedFolders().filter((x) => x.id !== folderId));
}

export function isPinnedFolder(folderId: string): boolean {
  return loadPinnedFolders().some((x) => x.id === folderId);
}
