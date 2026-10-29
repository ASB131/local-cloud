const KEY = "localcloud.recentFiles";
const MAX = 10;

export type RecentFile = { id: string; name: string; folderId: string };

export function loadRecentFiles(): RecentFile[] {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as RecentFile[];
    return Array.isArray(parsed) ? parsed.filter((x) => x?.id && x?.name && x?.folderId) : [];
  } catch {
    return [];
  }
}

export function pushRecentFile(entry: RecentFile) {
  if (!entry.id) return;
  const list = loadRecentFiles().filter((f) => f.id !== entry.id);
  list.unshift(entry);
  sessionStorage.setItem(KEY, JSON.stringify(list.slice(0, MAX)));
}
