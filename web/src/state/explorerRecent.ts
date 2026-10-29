const KEY = "localcloud.recentFolders";
const MAX = 8;

export type RecentFolder = { id: string; name: string };

export function loadRecentFolders(): RecentFolder[] {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as RecentFolder[];
    return Array.isArray(parsed) ? parsed.filter((x) => x?.id && x?.name) : [];
  } catch {
    return [];
  }
}

export function pushRecentFolder(entry: RecentFolder, rootFolderId: string) {
  if (!entry.id || entry.id === rootFolderId) return;
  const list = loadRecentFolders().filter((f) => f.id !== entry.id);
  list.unshift({ id: entry.id, name: entry.name });
  sessionStorage.setItem(KEY, JSON.stringify(list.slice(0, MAX)));
}
