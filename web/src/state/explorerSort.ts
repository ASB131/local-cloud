import type { CloudFile, Folder } from "@/api/types";
import { fileTypeLabel } from "@/lib/fileIcon";

export type SortKey = "name" | "size" | "modified" | "type";
export type SortState = { key: SortKey; dir: "asc" | "desc" };

const KEY = "localcloud.explorer.sort.v1";

export function loadSortState(): SortState {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return { key: "name", dir: "asc" };
    const data = JSON.parse(raw) as SortState;
    if (data.key !== "name" && data.key !== "size" && data.key !== "modified" && data.key !== "type") {
      return { key: "name", dir: "asc" };
    }
    if (data.dir !== "asc" && data.dir !== "desc") return { key: data.key, dir: "asc" };
    return data;
  } catch {
    return { key: "name", dir: "asc" };
  }
}

export function saveSortState(state: SortState) {
  sessionStorage.setItem(KEY, JSON.stringify(state));
}

function cmpStr(a: string, b: string, dir: SortState["dir"]) {
  const r = a.localeCompare(b, undefined, { sensitivity: "base" });
  return dir === "asc" ? r : -r;
}

function cmpNum(a: number, b: number, dir: SortState["dir"]) {
  return dir === "asc" ? a - b : b - a;
}

export function toggleSort(current: SortState, key: SortKey): SortState {
  if (current.key === key) {
    return { key, dir: current.dir === "asc" ? "desc" : "asc" };
  }
  return { key, dir: key === "name" ? "asc" : "desc" };
}

export function sortFolders(folders: Folder[], sort: SortState): Folder[] {
  const list = [...folders];
  list.sort((a, b) => {
    if (sort.key === "name" || sort.key === "size" || sort.key === "type") {
      return cmpStr(a.name, b.name, sort.dir);
    }
    return cmpStr(a.updated_at, b.updated_at, sort.dir);
  });
  return list;
}

export function sortFiles(files: CloudFile[], sort: SortState): CloudFile[] {
  const list = [...files];
  const modified = (f: CloudFile) => f.meta_modified_at ?? f.updated_at;
  list.sort((a, b) => {
    if (sort.key === "name") return cmpStr(a.name, b.name, sort.dir);
    if (sort.key === "size") return cmpNum(a.size_bytes, b.size_bytes, sort.dir);
    if (sort.key === "type") return cmpStr(fileTypeLabel(a), fileTypeLabel(b), sort.dir);
    return cmpStr(modified(a), modified(b), sort.dir);
  });
  return list;
}

export function sortIndicator(active: SortKey, sort: SortState) {
  if (sort.key !== active) return "";
  return sort.dir === "asc" ? " ▲" : " ▼";
}
