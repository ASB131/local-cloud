import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import {
  addFavorite,
  copyFile,
  copyFolder,
  createFolder,
  deleteFile,
  deleteFolder,
  downloadArchive,
  downloadFile,
  getFile,
  getFolder,
  getFolderChildren,
  listFilesByCategory,
  listFavorites,
  patchFile,
  patchFolder,
  removeFavorite,
} from "@/api/files";
import type { Folder, CloudFile } from "@/api/types";
import { CollisionModal } from "@/components/CollisionModal";
import { ContextMenu, type ContextMenuItem } from "@/components/ContextMenu";
import { MoveToDialog } from "@/components/MoveToDialog";
import { PreviewDrawer } from "@/components/PreviewDrawer";
import { PromptDialog } from "@/components/PromptDialog";
import { ExplorerFileIcon } from "@/components/ExplorerFileIcon";
import { ExplorerFolderIcon } from "@/components/ExplorerFolderIcon";
import { InlineRenameInput } from "@/components/InlineRenameInput";
import {
  fileDisplayName,
  fileNameFromRenameEdit,
  fileRenameEditValue,
} from "@/lib/fileDisplayName";
import { PropertiesDialog } from "@/components/PropertiesDialog";
import {
  clearCutItems,
  getClipboard,
  hasClipboardItems,
  setCopyItems,
  setCutItems,
} from "@/state/explorerClipboard";
import { isExplorerDrag, readExplorerDrag, writeExplorerDrag, type ExplorerSel } from "@/state/explorerDrag";
import { useExplorerNav } from "@/state/explorerNav";
import {
  loadSortState,
  saveSortState,
  sortFiles,
  sortFolders,
  sortIndicator,
  toggleSort,
  type SortState,
} from "@/state/explorerSort";
import { fileCategoryLabel } from "@/lib/fileCategoriesMeta";
import { formatBytes } from "@/lib/formatBytes";
import { FOLDER_TYPE_LABEL, fileTypeLabel } from "@/lib/fileIcon";
import { resolveFolderPath } from "@/lib/resolveFolderPath";
import { useCtrlWheelGridSize } from "@/hooks/useCtrlWheelGridSize";
import { loadGridSize, saveGridSize, type GridSize } from "@/state/explorerGridSize";
import { pushRecentFolder } from "@/state/explorerRecent";
import { isPinnedFolder, pinFolder, unpinFolder } from "@/state/explorerPinned";
import { pushRecentFile } from "@/state/explorerRecentFiles";
import { useAuth } from "@/state/auth";
import { useTransfers } from "@/state/transfersContext";

type Sel = ExplorerSel;

type MenuState =
  | { kind: "background"; x: number; y: number }
  | { kind: "items"; x: number; y: number; selection: Sel[] };

function rectsIntersect(a: DOMRect, b: { left: number; top: number; right: number; bottom: number }) {
  return !(a.right < b.left || a.left > b.right || a.bottom < b.top || a.top > b.bottom);
}

function pointerToLocal(host: HTMLElement, clientX: number, clientY: number) {
  const rect = host.getBoundingClientRect();
  return {
    x: clientX - rect.left + host.scrollLeft,
    y: clientY - rect.top + host.scrollTop,
  };
}

function localRectToClient(
  host: HTMLElement,
  box: { left: number; top: number; right: number; bottom: number },
) {
  const rect = host.getBoundingClientRect();
  const scrollLeft = host.scrollLeft;
  const scrollTop = host.scrollTop;
  return {
    left: rect.left + box.left - scrollLeft,
    top: rect.top + box.top - scrollTop,
    right: rect.left + box.right - scrollLeft,
    bottom: rect.top + box.bottom - scrollTop,
  };
}

export function CloudFolderPage() {
  const { user } = useAuth();
  const { folderId: paramId } = useParams();
  const folderId = paramId ?? user?.root_folder_id ?? "";
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const qc = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);
  const selectHostRef = useRef<HTMLDivElement>(null);
  const addressInputRef = useRef<HTMLInputElement>(null);
  const skipAddressBlur = useRef(false);
  const { enqueueFromFileList, enqueueFromDataTransfer, items, resolveConflict } = useTransfers();
  const explorerNav = useExplorerNav();
  const rootFolderId = user?.root_folder_id ?? "";
  const categoryId = searchParams.get("category");
  const categoryMode = Boolean(categoryId);

  const [view, setView] = useState<"list" | "grid">("grid");
  const [gridSize, setGridSize] = useState<GridSize>(() => loadGridSize());
  useCtrlWheelGridSize(selectHostRef, view === "grid", gridSize, setGridSize);
  const [sort, setSort] = useState<SortState>(() => loadSortState());
  const [selected, setSelected] = useState<Sel[]>([]);
  const [anchorIndex, setAnchorIndex] = useState(0);
  const [preview, setPreview] = useState<CloudFile | null>(null);
  const [properties, setProperties] = useState<
    { kind: "folder"; folder: Folder } | { kind: "file"; file: CloudFile } | null
  >(null);
  const [folderSearchDraft, setFolderSearchDraft] = useState("");
  const [dragOver, setDragOver] = useState(false);
  const [internalDropTarget, setInternalDropTarget] = useState<string | null>(null);
  const [favoritesOnly, setFavoritesOnly] = useState(false);
  const [prompt, setPrompt] = useState<null | { kind: "folder" }>(null);
  const [inlineRename, setInlineRename] = useState<null | { sel: Sel; value: string }>(null);
  const [addressEdit, setAddressEdit] = useState(false);
  const [addressDraft, setAddressDraft] = useState("");
  const [addressError, setAddressError] = useState<string | null>(null);
  const [moveOpen, setMoveOpen] = useState(false);
  const [moveError, setMoveError] = useState<string | null>(null);
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [clipRev, setClipRev] = useState(0);
  const [marquee, setMarquee] = useState<null | { x1: number; y1: number; x2: number; y2: number }>(null);
  const marqueeRef = useRef<null | { x1: number; y1: number; x2: number; y2: number }>(null);

  const meta = useQuery({
    queryKey: ["folder", folderId],
    queryFn: () => getFolder(folderId),
    enabled: Boolean(folderId),
  });

  const children = useQuery({
    queryKey: ["folder", "children", folderId],
    queryFn: () => getFolderChildren(folderId),
    enabled: Boolean(folderId) && !categoryMode,
  });

  const categoryFiles = useQuery({
    queryKey: ["file-category", categoryId],
    queryFn: () => listFilesByCategory(categoryId!),
    enabled: categoryMode && Boolean(categoryId),
  });

  const conflict = items.find((t) => t.status === "conflict");
  const crumbs = meta.data?.breadcrumb ?? [];

  const rawFolders = useMemo(() => {
    if (categoryMode) return [];
    const list = children.data?.folders ?? [];
    return favoritesOnly ? [] : list;
  }, [categoryMode, children.data, favoritesOnly]);

  const rawFiles = useMemo(() => {
    if (categoryMode) return categoryFiles.data ?? [];
    const list = children.data?.files ?? [];
    return favoritesOnly ? list.filter((f) => f.is_favorite) : list;
  }, [categoryMode, categoryFiles.data, children.data, favoritesOnly]);

  const folders = useMemo(() => sortFolders(rawFolders, sort), [rawFolders, sort]);
  const files = useMemo(() => sortFiles(rawFiles, sort), [rawFiles, sort]);

  const flatOrder: Sel[] = useMemo(
    () => [...folders.map((f) => ({ kind: "folder" as const, id: f.id })), ...files.map((f) => ({ kind: "file" as const, id: f.id }))],
    [folders, files],
  );

  const folderHref = (id: string) => `/cloud/folder/${id}`;

  const goToFolder = (id: string) => {
    navigate(folderHref(id));
  };

  const pathLabel = useMemo(() => {
    if (categoryMode && categoryId) {
      return `Cloud › ${fileCategoryLabel(categoryId)}`;
    }
    const parts = crumbs.map((c) => c.name || "Root");
    if (meta.data?.folder?.parent_id) parts.push(meta.data.folder.name);
    return parts.join(" › ") || "Cloud";
  }, [categoryMode, categoryId, crumbs, meta.data?.folder]);

  useEffect(() => {
    if (folderId) explorerNav.push(folderId);
  }, [folderId, explorerNav]);

  useEffect(() => {
    const folder = meta.data?.folder;
    if (!folderId || !folder) return;
    const name =
      folderId === rootFolderId
        ? "Cloud"
        : folder.name || crumbs[crumbs.length - 1]?.name || "Folder";
    pushRecentFolder({ id: folderId, name }, rootFolderId);
  }, [folderId, rootFolderId, meta.data?.folder, crumbs]);

  useEffect(() => {
    setInlineRename(null);
    setAddressEdit(false);
    setAddressError(null);
  }, [folderId]);

  useEffect(() => {
    if (!addressEdit) return;
    setAddressDraft(pathLabel);
    const t = window.setTimeout(() => {
      addressInputRef.current?.focus();
      addressInputRef.current?.select();
    }, 0);
    return () => window.clearTimeout(t);
  }, [addressEdit, pathLabel]);

  const submitAddressPath = async () => {
    if (!rootFolderId || !addressEdit) return;
    setAddressError(null);
    const id = await resolveFolderPath(rootFolderId, addressDraft);
    if (!id) {
      setAddressError("Path not found.");
      return;
    }
    setAddressEdit(false);
    goToFolder(id);
  };

  const onSortColumn = (key: SortState["key"]) => {
    setSort((prev) => {
      const next = toggleSort(prev, key);
      saveSortState(next);
      return next;
    });
  };

  const formatModified = (iso: string) => {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" });
  };

  const invalidate = () => {
    if (categoryMode && categoryId) {
      qc.invalidateQueries({ queryKey: ["file-category", categoryId] });
    } else {
      qc.invalidateQueries({ queryKey: ["folder"] });
    }
    qc.invalidateQueries({ queryKey: ["favorites"] });
  };

  const listLoading = categoryMode ? categoryFiles.isLoading : children.isLoading;

  const createFolderMut = useMutation({
    mutationFn: (name: string) => createFolder(folderId, name),
    onSuccess: invalidate,
  });

  const isSelected = (sel: Sel) => selected.some((s) => s.kind === sel.kind && s.id === sel.id);

  const selectRange = (from: number, to: number) => {
    const a = Math.min(from, to);
    const b = Math.max(from, to);
    setSelected(flatOrder.slice(a, b + 1));
  };

  const handleItemClick = (sel: Sel, index: number, e: React.MouseEvent) => {
    if (e.shiftKey) {
      selectRange(anchorIndex, index);
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      setSelected((prev) =>
        isSelected(sel) ? prev.filter((p) => !(p.kind === sel.kind && p.id === sel.id)) : [...prev, sel],
      );
      setAnchorIndex(index);
      return;
    }
    setSelected([sel]);
    setAnchorIndex(index);
  };

  const selectedFolderIds = selected.filter((s) => s.kind === "folder").map((s) => s.id);
  const selectedFileIds = selected.filter((s) => s.kind === "file").map((s) => s.id);
  const singleSel = selected.length === 1 ? selected[0]! : null;

  const [downloadBusy, setDownloadBusy] = useState(false);

  const onDownload = async (sel: Sel[] = selected) => {
    const fIds = sel.filter((s) => s.kind === "file").map((s) => s.id);
    const dIds = sel.filter((s) => s.kind === "folder").map((s) => s.id);
    if (sel.length === 0) return;
    setDownloadBusy(true);
    try {
      if (fIds.length === 1 && dIds.length === 0) {
        const f = files.find((x) => x.id === fIds[0]);
        if (f) await downloadFile(f.id, f.name);
        return;
      }
      await downloadArchive(dIds, fIds);
    } finally {
      setDownloadBusy(false);
    }
  };

  const onDelete = async (sel: Sel[] = selected) => {
    for (const s of sel) {
      if (s.kind === "file") await deleteFile(s.id);
      else await deleteFolder(s.id);
    }
    setSelected([]);
    invalidate();
  };

  const moveItems = async (sel: Sel[], destFolderId: string) => {
    for (const s of sel) {
      if (s.kind === "file") await patchFile(s.id, { folder_id: destFolderId });
      else if (s.id !== destFolderId) await patchFolder(s.id, { parent_id: destFolderId });
    }
    setSelected([]);
    invalidate();
  };

  const applyMove = async (destFolderId: string) => {
    if (selected.length === 0) return;
    setMoveError(null);
    try {
      await moveItems(selected, destFolderId);
      setMoveOpen(false);
    } catch {
      setMoveError("Move failed. The destination may be invalid or would create a cycle.");
    }
  };

  const pasteClipboard = async () => {
    const clip = getClipboard();
    if (!clip?.items.length) return;
    if (clip.mode === "cut") {
      try {
        await moveItems(clip.items, folderId);
        clearCutItems();
        setClipRev((v) => v + 1);
      } catch {
        /* keep cut clipboard */
      }
      return;
    }
    try {
      for (const s of clip.items) {
        if (s.kind === "file") await copyFile(s.id, folderId);
        else await copyFolder(s.id, folderId);
      }
      invalidate();
    } catch {
      /* partial copy may have occurred */
      invalidate();
    }
  };

  const dragPayloadFor = (sel: Sel) =>
    selected.some((s) => s.kind === sel.kind && s.id === sel.id) && selected.length > 0 ? selected : [sel];

  const dropMoveOntoFolder = async (targetFolderId: string, dragItems: Sel[]) => {
    const toMove = dragItems.filter((s) => {
      if (s.kind === "folder" && s.id === targetFolderId) return false;
      if (s.kind === "file") {
        const f = files.find((x) => x.id === s.id);
        return f?.folder_id !== targetFolderId;
      }
      const f = folders.find((x) => x.id === s.id);
      return f?.parent_id !== targetFolderId;
    });
    if (toMove.length === 0) return;
    try {
      await moveItems(toMove, targetFolderId);
    } catch {
      /* move failed */
    }
  };

  const onItemDragStart = (sel: Sel) => (e: React.DragEvent) => {
    const payload = dragPayloadFor(sel);
    writeExplorerDrag(e.dataTransfer, payload);
    if (!isSelected(sel)) setSelected(payload);
  };

  const onFolderDragOver = (folderId: string) => (e: React.DragEvent) => {
    if (!isExplorerDrag(e.dataTransfer.types)) return;
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "move";
    setInternalDropTarget(folderId);
  };

  const onFolderDragLeave = (folderId: string) => (e: React.DragEvent) => {
    const related = e.relatedTarget as Node | null;
    if (related && e.currentTarget.contains(related)) return;
    setInternalDropTarget((id) => (id === folderId ? null : id));
  };

  const onFolderDrop = (folderId: string) => (e: React.DragEvent) => {
    if (!isExplorerDrag(e.dataTransfer.types)) return;
    e.preventDefault();
    e.stopPropagation();
    setInternalDropTarget(null);
    setDragOver(false);
    const payload = readExplorerDrag(e.dataTransfer);
    if (payload.length) void dropMoveOntoFolder(folderId, payload);
  };

  const renameTargetName = (sel: Sel) =>
    sel.kind === "file" ? files.find((f) => f.id === sel.id)?.name : folders.find((f) => f.id === sel.id)?.name;

  const isRenaming = (sel: Sel) =>
    inlineRename != null && inlineRename.sel.kind === sel.kind && inlineRename.sel.id === sel.id;

  const commitInlineRename = async () => {
    if (!inlineRename) return;
    const { sel, value } = inlineRename;
    const original = renameTargetName(sel) ?? "";
    const name =
      sel.kind === "file" ? fileNameFromRenameEdit(value, original) : value.trim();
    setInlineRename(null);
    if (!name || name === original) return;
    try {
      if (sel.kind === "file") await patchFile(sel.id, { name });
      else await patchFolder(sel.id, { name });
      invalidate();
    } catch {
      setInlineRename({ sel, value: name });
    }
  };

  const onFavorite = async (file: CloudFile) => {
    if (file.is_favorite) {
      const favs = await listFavorites();
      const fav = favs.find((f) => f.file_id === file.id);
      if (fav) await removeFavorite(fav.id);
    } else {
      await addFavorite({ file_id: file.id });
    }
    invalidate();
  };

  const startRename = (sel: Sel[]) => {
    if (sel.length !== 1) return;
    const s = sel[0]!;
    setSelected(sel);
    const full = renameTargetName(s) ?? "";
    setInlineRename({
      sel: s,
      value: s.kind === "file" ? fileRenameEditValue(full) : full,
    });
  };

  const openItem = (sel: Sel, newTab: boolean) => {
    if (sel.kind === "folder") {
      if (newTab) window.open(folderHref(sel.id), "_blank", "noopener,noreferrer");
      else goToFolder(sel.id);
    } else {
      const f = files.find((x) => x.id === sel.id);
      if (f) setPreview(f);
    }
  };

  const showProperties = (sel: Sel) => {
    if (sel.kind === "file") {
      const f = files.find((x) => x.id === sel.id);
      if (f) setProperties({ kind: "file", file: f });
    } else {
      const f = folders.find((x) => x.id === sel.id);
      if (f) setProperties({ kind: "folder", folder: f });
    }
  };

  const selectionStats = useMemo(() => {
    let bytes = 0;
    let filesN = 0;
    let foldersN = 0;
    for (const s of selected) {
      if (s.kind === "file") {
        filesN += 1;
        const f = files.find((x) => x.id === s.id);
        if (f) bytes += f.size_bytes;
      } else foldersN += 1;
    }
    return { bytes, filesN, foldersN };
  }, [selected, files]);

  const buildItemMenu = (sel: Sel[]): ContextMenuItem[] => {
    const one = sel.length === 1 ? sel[0]! : null;
    const file = one?.kind === "file" ? files.find((f) => f.id === one.id) : null;
    const folder = one?.kind === "folder" ? folders.find((f) => f.id === one.id) : null;
    const canPaste = hasClipboardItems();
    return [
      {
        type: "item",
        id: "open",
        label: one?.kind === "folder" ? "Open" : "Preview",
        disabled: !one,
        onClick: () => one && openItem(one, false),
      },
      {
        type: "item",
        id: "open-tab",
        label: "Open in new tab",
        disabled: one?.kind !== "folder",
        onClick: () => one && openItem(one, true),
      },
      {
        type: "item",
        id: "pin-home",
        label: folder && isPinnedFolder(folder.id) ? "Unpin from Home" : "Pin to Home",
        disabled: !folder,
        onClick: () => {
          if (!folder) return;
          if (isPinnedFolder(folder.id)) unpinFolder(folder.id);
          else pinFolder({ id: folder.id, name: folder.name });
        },
      },
      { type: "separator" },
      {
        type: "item",
        id: "cut",
        label: "Cut",
        disabled: sel.length === 0,
        onClick: () => {
          setCutItems(sel);
          setClipRev((v) => v + 1);
        },
      },
      {
        type: "item",
        id: "copy",
        label: "Copy",
        disabled: sel.length === 0,
        onClick: () => {
          setCopyItems(sel);
          setClipRev((v) => v + 1);
        },
      },
      {
        type: "item",
        id: "paste",
        label: "Paste",
        disabled: !canPaste,
        onClick: () => void pasteClipboard(),
      },
      { type: "separator" },
      {
        type: "item",
        id: "download",
        label: "Download",
        disabled: sel.length === 0,
        onClick: () => void onDownload(sel),
      },
      {
        type: "item",
        id: "rename",
        label: "Rename",
        disabled: sel.length !== 1,
        onClick: () => startRename(sel),
      },
      {
        type: "item",
        id: "move",
        label: "Move to…",
        disabled: sel.length !== 1,
        onClick: () => {
          setSelected(sel);
          setMoveError(null);
          setMoveOpen(true);
        },
      },
      {
        type: "item",
        id: "delete",
        label: "Delete",
        danger: true,
        disabled: sel.length === 0,
        onClick: () => void onDelete(sel),
      },
      { type: "separator" },
      {
        type: "item",
        id: "favorite",
        label: file?.is_favorite ? "Remove from favorites" : "Add to favorites",
        disabled: !file,
        onClick: () => file && void onFavorite(file),
      },
      {
        type: "item",
        id: "versions",
        label: "Version history",
        disabled: !file,
        onClick: () => file && setPreview(file),
      },
      {
        type: "item",
        id: "properties",
        label: "Properties",
        disabled: sel.length !== 1,
        onClick: () => one && showProperties(one),
      },
    ];
  };

  const buildBackgroundMenu = (): ContextMenuItem[] => {
    void clipRev;
    return [
      { type: "item", id: "view-icons", label: "Icons view", onClick: () => setView("grid") },
      { type: "item", id: "view-list", label: "List view", onClick: () => setView("list") },
      ...(categoryMode
        ? []
        : ([
            { type: "separator" as const },
            {
              type: "item" as const,
              id: "paste-bg",
              label: "Paste",
              disabled: !hasClipboardItems(),
              onClick: () => void pasteClipboard(),
            },
            { type: "item" as const, id: "new-folder", label: "New folder", onClick: () => setPrompt({ kind: "folder" }) },
            { type: "item" as const, id: "upload", label: "Upload files", onClick: () => fileInputRef.current?.click() },
            {
              type: "item" as const,
              id: "upload-dir",
              label: "Upload folder",
              onClick: () => folderInputRef.current?.click(),
            },
          ] as const)),
      { type: "separator" },
      { type: "item", id: "refresh", label: "Refresh", onClick: () => invalidate() },
    ];
  };

  const onItemContextMenu = (e: React.MouseEvent, sel: Sel) => {
    e.preventDefault();
    e.stopPropagation();
    const selection = isSelected(sel) && selected.length > 0 ? selected : [sel];
    if (!isSelected(sel)) setSelected([sel]);
    setMenu({ kind: "items", x: e.clientX, y: e.clientY, selection });
  };

  const onBackgroundContextMenu = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest("[data-explorer-item]")) return;
    e.preventDefault();
    setMenu({ kind: "background", x: e.clientX, y: e.clientY });
  };

  const finishMarquee = useCallback(
    (box: { left: number; top: number; right: number; bottom: number }) => {
      const host = selectHostRef.current;
      if (!host) return;
      const hits: Sel[] = [];
      host.querySelectorAll("[data-explorer-item]").forEach((el) => {
        const rect = el.getBoundingClientRect();
        if (rectsIntersect(rect, box)) {
          const kind = el.getAttribute("data-kind");
          const id = el.getAttribute("data-id");
          if (kind === "folder" || kind === "file") hits.push({ kind, id: id! });
        }
      });
      if (hits.length) setSelected(hits);
    },
    [],
  );

  const previewFile = useMemo(
    () => (preview ? (files.find((f) => f.id === preview.id) ?? preview) : null),
    [preview, files],
  );

  useEffect(() => {
    if (!preview || listLoading || files.length === 0) return;
    if (!categoryMode && preview.folder_id !== folderId) return;
    if (!files.some((f) => f.id === preview.id)) {
      setPreview(null);
    }
  }, [preview, files, listLoading, folderId, categoryMode]);

  useEffect(() => {
    const previewId = searchParams.get("preview");
    if (!previewId) return;

    const finish = (f: CloudFile | null) => {
      if (f) setPreview(f);
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete("preview");
          return next;
        },
        { replace: true },
      );
    };

    if (listLoading) return;

    const inList = files.find((x) => x.id === previewId);
    if (inList) {
      finish(inList);
      return;
    }

    void getFile(previewId)
      .then((f) => {
        if (categoryMode && categoryId) {
          navigate(
            `/cloud/folder/${rootFolderId}?category=${encodeURIComponent(categoryId)}&preview=${encodeURIComponent(previewId)}`,
            { replace: true },
          );
          return;
        }
        if (f.folder_id !== folderId) {
          navigate(`/cloud/folder/${f.folder_id}?preview=${encodeURIComponent(previewId)}`, { replace: true });
          return;
        }
        finish(f);
      })
      .catch(() => finish(null));
  }, [
    categoryId,
    categoryMode,
    folderId,
    files,
    listLoading,
    navigate,
    rootFolderId,
    searchParams,
    setSearchParams,
  ]);

  useEffect(() => {
    if (!previewFile) return;
    pushRecentFile({
      id: previewFile.id,
      name: previewFile.name,
      folderId: previewFile.folder_id,
    });
  }, [previewFile?.id, previewFile?.name, previewFile?.folder_id]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (preview) return;
      const tag = (e.target as HTMLElement).tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;
      if (e.key === "Enter" && singleSel && !inlineRename) {
        e.preventDefault();
        openItem(singleSel, false);
      }
      if (e.key === " " && singleSel?.kind === "file" && !inlineRename) {
        e.preventDefault();
        openItem(singleSel, false);
      }
      if (e.key === "Backspace" && !e.altKey && !e.ctrlKey && !e.metaKey) {
        if (parentId) {
          e.preventDefault();
          goToFolder(parentId);
        }
      }
      if (e.altKey && e.key === "Enter" && singleSel && !inlineRename) {
        e.preventDefault();
        showProperties(singleSel);
      }
      if (e.key === "F5") {
        e.preventDefault();
        invalidate();
      }
      if (e.key === "Delete" && selected.length) {
        e.preventDefault();
        void onDelete();
      }
      if (e.key === "F2" && singleSel && !inlineRename) {
        e.preventDefault();
        startRename([singleSel]);
      }
      if (e.altKey && e.key === "ArrowLeft" && explorerNav.canBack) {
        e.preventDefault();
        const id = explorerNav.back();
        if (id) goToFolder(id);
      }
      if (e.altKey && e.key === "ArrowRight" && explorerNav.canForward) {
        e.preventDefault();
        const id = explorerNav.forward();
        if (id) goToFolder(id);
      }
      if (e.altKey && e.key.toLowerCase() === "d") {
        e.preventDefault();
        setAddressEdit(true);
      }
      if (e.ctrlKey || e.metaKey) {
        if (e.key === "a" || e.key === "A") {
          e.preventDefault();
          setSelected(flatOrder);
        }
        if (e.key === "x" || e.key === "X") {
          e.preventDefault();
          setCutItems(selected);
          setClipRev((v) => v + 1);
        }
        if (e.key === "c" || e.key === "C") {
          e.preventDefault();
          setCopyItems(selected);
          setClipRev((v) => v + 1);
        }
        if (e.key === "v" || e.key === "V") {
          e.preventDefault();
          void pasteClipboard();
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  useEffect(() => {
    if (!marquee) {
      marqueeRef.current = null;
      return;
    }
    marqueeRef.current = marquee;
    const onMove = (e: MouseEvent) => {
      const host = selectHostRef.current;
      if (!host) return;
      const p = pointerToLocal(host, e.clientX, e.clientY);
      setMarquee((m) => {
        const next = m ? { ...m, x2: p.x, y2: p.y } : null;
        marqueeRef.current = next;
        return next;
      });
    };
    const onUp = () => {
      const m = marqueeRef.current;
      const host = selectHostRef.current;
      marqueeRef.current = null;
      setMarquee(null);
      if (!m || !host) return;
      const local = {
        left: Math.min(m.x1, m.x2),
        top: Math.min(m.y1, m.y2),
        right: Math.max(m.x1, m.x2),
        bottom: Math.max(m.y1, m.y2),
      };
      if (Math.abs(m.x1 - m.x2) > 4 || Math.abs(m.y1 - m.y2) > 4) {
        finishMarquee(localRectToClient(host, local));
      } else setSelected([]);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [marquee !== null, finishMarquee]);

  const parentId = crumbs.length >= 2 ? crumbs[crumbs.length - 2]?.id : null;

  if (!folderId) {
    return <div className="explorer-content text-[#9a9a9a]">Loading…</div>;
  }

  const marqueeStyle = marquee
    ? {
        left: Math.min(marquee.x1, marquee.x2),
        top: Math.min(marquee.y1, marquee.y2),
        width: Math.abs(marquee.x2 - marquee.x1),
        height: Math.abs(marquee.y2 - marquee.y1),
      }
    : null;

  return (
    <>
      {moveOpen && user?.root_folder_id ? (
        <MoveToDialog
          rootFolderId={user.root_folder_id}
          excludeFolderId={singleSel?.kind === "folder" ? singleSel.id : undefined}
          onCancel={() => {
            setMoveOpen(false);
            setMoveError(null);
          }}
          onSelect={(id) => void applyMove(id)}
          error={moveError}
        />
      ) : null}
      {prompt?.kind === "folder" ? (
        <PromptDialog
          title="New folder"
          label="Name"
          submitLabel="Create"
          onCancel={() => setPrompt(null)}
          onSubmit={(name) => {
            createFolderMut.mutate(name);
            setPrompt(null);
          }}
        />
      ) : null}
      {properties ? (
        <PropertiesDialog target={properties} folderPathLabel={pathLabel} onClose={() => setProperties(null)} />
      ) : null}
      {menu ? (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          items={menu.kind === "background" ? buildBackgroundMenu() : buildItemMenu(menu.selection)}
          onClose={() => setMenu(null)}
        />
      ) : null}
      {conflict ? (
        <CollisionModal
          transfer={conflict}
          onCancel={() => resolveConflict(conflict.id, "cancel")}
          onOverwrite={() => resolveConflict(conflict.id, "overwrite")}
          onRename={(n) => resolveConflict(conflict.id, "rename", n)}
        />
      ) : null}
      <div className="explorer-titlebar">
        <div className="explorer-nav-buttons">
          <button
            type="button"
            className="explorer-btn explorer-icon-btn"
            title="Back (Alt+Left)"
            aria-label="Back"
            disabled={!explorerNav.canBack}
            onClick={() => {
              const id = explorerNav.back();
              if (id) goToFolder(id);
            }}
          >
            ←
          </button>
          <button
            type="button"
            className="explorer-btn explorer-icon-btn"
            title="Forward (Alt+Right)"
            aria-label="Forward"
            disabled={!explorerNav.canForward}
            onClick={() => {
              const id = explorerNav.forward();
              if (id) goToFolder(id);
            }}
          >
            →
          </button>
          <button
            type="button"
            className="explorer-btn explorer-icon-btn"
            title="Up (Backspace)"
            aria-label="Up to parent folder"
            disabled={!parentId}
            onClick={() => parentId && goToFolder(parentId)}
          >
            ↑
          </button>
        </div>
        <div className="explorer-address-wrap">
          {addressEdit ? (
            <input
              ref={addressInputRef}
              type="text"
              className="explorer-address-input"
              value={addressDraft}
              aria-label="Folder path"
              onChange={(e) => {
                setAddressDraft(e.target.value);
                setAddressError(null);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  void submitAddressPath();
                }
                if (e.key === "Escape") {
                  e.preventDefault();
                  skipAddressBlur.current = true;
                  setAddressEdit(false);
                  setAddressError(null);
                }
              }}
              onBlur={() => {
                if (skipAddressBlur.current) {
                  skipAddressBlur.current = false;
                  return;
                }
                void submitAddressPath();
              }}
            />
          ) : (
            <div
              className="explorer-address"
              title="Double-click or Alt+D to edit path"
              onDoubleClick={(e) => {
                if ((e.target as HTMLElement).closest("a")) return;
                setAddressEdit(true);
              }}
            >
              <span
                className={`explorer-crumb${internalDropTarget === rootFolderId ? " crumb-drop-target" : ""}`}
                onDragOver={onFolderDragOver(rootFolderId)}
                onDragLeave={onFolderDragLeave(rootFolderId)}
                onDrop={onFolderDrop(rootFolderId)}
              >
                <Link to="/cloud">Home</Link>
              </span>
              {categoryMode && categoryId ? (
                <>
                  <span className="text-[#6e6e6e]">›</span>
                  <span className="explorer-crumb">
                    <Link to={`/cloud/folder/${rootFolderId}?category=${encodeURIComponent(categoryId)}`}>
                      {fileCategoryLabel(categoryId)}
                    </Link>
                  </span>
                </>
              ) : null}
              {!categoryMode &&
              crumbs.map((c: Folder) => (
                <span key={c.id} className="flex items-center gap-1">
                  <span className="text-[#6e6e6e]">›</span>
                  <span
                    className={`explorer-crumb${internalDropTarget === c.id ? " crumb-drop-target" : ""}`}
                    onDragOver={onFolderDragOver(c.id)}
                    onDragLeave={onFolderDragLeave(c.id)}
                    onDrop={onFolderDrop(c.id)}
                  >
                    <Link to={folderHref(c.id)}>{c.name || "Root"}</Link>
                  </span>
                </span>
              ))}
              {!categoryMode && meta.data?.folder?.parent_id ? (
                <>
                  <span className="text-[#6e6e6e]">›</span>
                  <span
                    className={`explorer-crumb${internalDropTarget === folderId ? " crumb-drop-target" : ""}`}
                    onDragOver={onFolderDragOver(folderId)}
                    onDragLeave={onFolderDragLeave(folderId)}
                    onDrop={onFolderDrop(folderId)}
                  >
                    {meta.data.folder.name}
                  </span>
                </>
              ) : null}
            </div>
          )}
          {addressError ? <div className="explorer-address-error">{addressError}</div> : null}
        </div>
        <input
          className="explorer-search"
          placeholder="Search all files (Enter)"
          value={folderSearchDraft}
          onChange={(e) => setFolderSearchDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              const q = folderSearchDraft.trim();
              if (q) navigate(`/cloud/search?q=${encodeURIComponent(q)}`);
            }
          }}
        />
      </div>

      <div className="explorer-command">
        {!categoryMode ? (
          <>
            <div className="explorer-command-group">
              <button type="button" className="explorer-btn" onClick={() => setPrompt({ kind: "folder" })}>
                New folder
              </button>
              <button type="button" className="explorer-btn explorer-btn-primary" onClick={() => fileInputRef.current?.click()}>
                Upload
              </button>
              <button type="button" className="explorer-btn" onClick={() => folderInputRef.current?.click()}>
                Upload folder
              </button>
            </div>
            <span className="explorer-command-sep" aria-hidden />
          </>
        ) : null}
        <div className="explorer-command-group">
          <button
            type="button"
            className="explorer-btn"
            disabled={!singleSel}
            title={singleSel?.kind === "folder" ? "Open (Enter)" : "Preview (Enter or Space)"}
            onClick={() => singleSel && openItem(singleSel, false)}
          >
            {singleSel?.kind === "folder" ? "Open" : "Preview"}
          </button>
          <button type="button" className="explorer-btn" disabled={selected.length === 0} onClick={() => void onDownload()}>
            Download
          </button>
          <button
            type="button"
            className="explorer-btn"
            disabled={selected.length !== 1}
            onClick={() => singleSel && startRename([singleSel])}
          >
            Rename
          </button>
          <button
            type="button"
            className="explorer-btn"
            disabled={selected.length !== 1}
            onClick={() => {
              setMoveError(null);
              setMoveOpen(true);
            }}
          >
            Move to
          </button>
          <button type="button" className="explorer-btn" disabled={selected.length === 0} onClick={() => void onDelete()}>
            Delete
          </button>
        </div>
        <span className="explorer-command-sep" aria-hidden />
        <div className="explorer-command-group">
          <button type="button" className="explorer-btn" title="Refresh (F5)" onClick={() => invalidate()}>
            Refresh
          </button>
          <button type="button" className="explorer-btn" onClick={() => setView(view === "grid" ? "list" : "grid")}>
            {view === "grid" ? "List view" : "Icons"}
          </button>
          {view === "grid" ? (
            <span className="explorer-icon-size-group" role="group" aria-label="Icon size">
              {(["sm", "md", "lg"] as const).map((size) => (
                <button
                  key={size}
                  type="button"
                  className={`explorer-btn explorer-btn-compact ${gridSize === size ? "active" : ""}`}
                  title={size === "sm" ? "Small icons" : size === "md" ? "Medium icons" : "Large icons"}
                  onClick={() => {
                    setGridSize(size);
                    saveGridSize(size);
                  }}
                >
                  {size === "sm" ? "S" : size === "md" ? "M" : "L"}
                </button>
              ))}
            </span>
          ) : null}
          <button
            type="button"
            className={`explorer-btn ${favoritesOnly ? "active" : ""}`}
            onClick={() => setFavoritesOnly((v) => !v)}
          >
            Favorites only
          </button>
        </div>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          className="hidden"
          onChange={(e) => {
            if (e.target.files?.length) enqueueFromFileList(folderId, e.target.files);
            e.target.value = "";
          }}
        />
        <input
          ref={folderInputRef}
          type="file"
          multiple
          className="hidden"
          // @ts-expect-error webkitdirectory
          webkitdirectory=""
          onChange={(e) => {
            if (e.target.files?.length) enqueueFromFileList(folderId, e.target.files);
            e.target.value = "";
          }}
        />
      </div>

      <div
        className={`explorer-content-select-host ${dragOver ? "drag-over" : ""}`}
        onDragOver={(e) => {
          if (isExplorerDrag(e.dataTransfer.types)) {
            e.preventDefault();
            return;
          }
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          if (isExplorerDrag(e.dataTransfer.types)) return;
          e.preventDefault();
          setDragOver(false);
          void enqueueFromDataTransfer(folderId, e.dataTransfer);
        }}
      >
        <div
          ref={selectHostRef}
          className={`explorer-content explorer-file-pane ${dragOver ? "drag-over" : ""}`}
          onContextMenu={onBackgroundContextMenu}
          onMouseDown={(e) => {
            if (e.button !== 0 || (e.target as HTMLElement).closest("[data-explorer-item]")) return;
            const host = selectHostRef.current;
            if (!host) return;
            const p = pointerToLocal(host, e.clientX, e.clientY);
            setMarquee({ x1: p.x, y1: p.y, x2: p.x, y2: p.y });
          }}
        >
        {marqueeStyle ? <div className="explorer-marquee" style={marqueeStyle} /> : null}

        {listLoading ? <p className="text-[#9a9a9a]">Loading…</p> : null}
        {children.error ? <p className="text-red-400">Could not load this folder.</p> : null}

        {!listLoading && folders.length === 0 && files.length === 0 ? (
          <p className="explorer-empty-hint">
            {categoryMode ? "No files in this category." : "This folder is empty. Drag files here or choose Upload."}
          </p>
        ) : null}

        {view === "grid" ? (
          <div className={`explorer-grid explorer-grid--${gridSize}`}>
            {folders.map((f, i) => (
              <div
                key={f.id}
                data-explorer-item
                data-kind="folder"
                data-id={f.id}
                role="button"
                tabIndex={0}
                draggable={!isRenaming({ kind: "folder", id: f.id })}
                className={`explorer-tile ${isSelected({ kind: "folder", id: f.id }) ? "selected" : ""} ${internalDropTarget === f.id ? "drop-target" : ""}`}
                onClick={(e) => {
                  if (inlineRename) return;
                  handleItemClick({ kind: "folder", id: f.id }, i, e);
                }}
                onDoubleClick={() => goToFolder(f.id)}
                onContextMenu={(e) => onItemContextMenu(e, { kind: "folder", id: f.id })}
                onDragStart={onItemDragStart({ kind: "folder", id: f.id })}
                onDragEnd={() => setInternalDropTarget(null)}
                onDragOver={onFolderDragOver(f.id)}
                onDragLeave={onFolderDragLeave(f.id)}
                onDrop={onFolderDrop(f.id)}
              >
                <ExplorerFolderIcon variant="grid" />
                {isRenaming({ kind: "folder", id: f.id }) ? (
                  <InlineRenameInput
                    className="explorer-inline-rename explorer-tile-name"
                    value={inlineRename!.value}
                    onChange={(value) => setInlineRename({ sel: inlineRename!.sel, value })}
                    onCommit={() => void commitInlineRename()}
                    onCancel={() => setInlineRename(null)}
                  />
                ) : (
                  <div className="explorer-tile-name">{f.name}</div>
                )}
              </div>
            ))}
            {files.map((f, i) => {
              const index = folders.length + i;
              return (
                <div
                  key={f.id}
                  data-explorer-item
                  data-kind="file"
                  data-id={f.id}
                  role="button"
                  tabIndex={0}
                  draggable={!isRenaming({ kind: "file", id: f.id })}
                  className={`explorer-tile ${isSelected({ kind: "file", id: f.id }) ? "selected" : ""}`}
                  onClick={(e) => {
                    if (inlineRename) return;
                    handleItemClick({ kind: "file", id: f.id }, index, e);
                  }}
                  onDoubleClick={() => setPreview(f)}
                  onContextMenu={(e) => onItemContextMenu(e, { kind: "file", id: f.id })}
                  onDragStart={onItemDragStart({ kind: "file", id: f.id })}
                  onDragEnd={() => setInternalDropTarget(null)}
                >
                  <ExplorerFileIcon file={f} variant="grid" />
                  {isRenaming({ kind: "file", id: f.id }) ? (
                    <InlineRenameInput
                      className="explorer-inline-rename explorer-tile-name"
                      value={inlineRename!.value}
                      onChange={(value) => setInlineRename({ sel: inlineRename!.sel, value })}
                      onCommit={() => void commitInlineRename()}
                      onCancel={() => setInlineRename(null)}
                    />
                  ) : (
                    <div className="explorer-tile-name" title={f.name}>
                      {fileDisplayName(f.name)}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        ) : (
          <div className="explorer-list">
            <div className="explorer-list-row explorer-list-header text-xs text-[#9a9a9a]">
              <button type="button" className="explorer-col-sort" onClick={() => onSortColumn("name")}>
                Name{sortIndicator("name", sort)}
              </button>
              <button type="button" className="explorer-col-sort" onClick={() => onSortColumn("modified")}>
                Date modified{sortIndicator("modified", sort)}
              </button>
              <button type="button" className="explorer-col-sort" onClick={() => onSortColumn("type")}>
                Type{sortIndicator("type", sort)}
              </button>
              <button type="button" className="explorer-col-sort explorer-col-sort--end" onClick={() => onSortColumn("size")}>
                Size{sortIndicator("size", sort)}
              </button>
              <span aria-hidden />
            </div>
            {folders.map((f, i) => (
              <div
                key={f.id}
                data-explorer-item
                data-kind="folder"
                data-id={f.id}
                draggable={!isRenaming({ kind: "folder", id: f.id })}
                className={`explorer-list-row ${isSelected({ kind: "folder", id: f.id }) ? "selected" : ""} ${internalDropTarget === f.id ? "drop-target" : ""}`}
                onDoubleClick={() => !inlineRename && goToFolder(f.id)}
                onContextMenu={(e) => onItemContextMenu(e, { kind: "folder", id: f.id })}
                onClick={(e) => {
                  if (inlineRename) return;
                  handleItemClick({ kind: "folder", id: f.id }, i, e);
                }}
                onDragStart={onItemDragStart({ kind: "folder", id: f.id })}
                onDragEnd={() => setInternalDropTarget(null)}
                onDragOver={onFolderDragOver(f.id)}
                onDragLeave={onFolderDragLeave(f.id)}
                onDrop={onFolderDrop(f.id)}
              >
                {isRenaming({ kind: "folder", id: f.id }) ? (
                  <InlineRenameInput
                    className="explorer-inline-rename min-w-0"
                    value={inlineRename!.value}
                    onChange={(value) => setInlineRename({ sel: inlineRename!.sel, value })}
                    onCommit={() => void commitInlineRename()}
                    onCancel={() => setInlineRename(null)}
                  />
                ) : (
                  <span className="explorer-list-name truncate">
                    <ExplorerFolderIcon variant="inline" />
                    <span className="truncate">{f.name}</span>
                  </span>
                )}
                <span className="text-[#6e6e6e] tabular-nums">{formatModified(f.updated_at)}</span>
                <span className="truncate text-[#6e6e6e]">{FOLDER_TYPE_LABEL}</span>
                <span className="text-[#6e6e6e] tabular-nums text-right">—</span>
                <span />
              </div>
            ))}
            {files.map((f, i) => {
              const index = folders.length + i;
              return (
                <div
                  key={f.id}
                  data-explorer-item
                  data-kind="file"
                  data-id={f.id}
                  draggable={!isRenaming({ kind: "file", id: f.id })}
                  className={`explorer-list-row ${isSelected({ kind: "file", id: f.id }) ? "selected" : ""}`}
                  onDoubleClick={() => !inlineRename && setPreview(f)}
                  onContextMenu={(e) => onItemContextMenu(e, { kind: "file", id: f.id })}
                  onClick={(e) => {
                    if (inlineRename) return;
                    handleItemClick({ kind: "file", id: f.id }, index, e);
                  }}
                  onDragStart={onItemDragStart({ kind: "file", id: f.id })}
                  onDragEnd={() => setInternalDropTarget(null)}
                >
                  {isRenaming({ kind: "file", id: f.id }) ? (
                    <InlineRenameInput
                      className="explorer-inline-rename min-w-0"
                      value={inlineRename!.value}
                      onChange={(value) => setInlineRename({ sel: inlineRename!.sel, value })}
                      onCommit={() => void commitInlineRename()}
                      onCancel={() => setInlineRename(null)}
                    />
                  ) : (
                    <span className="explorer-list-name truncate">
                      <ExplorerFileIcon file={f} variant="inline" />
                      <span className="truncate" title={f.name}>
                        {fileDisplayName(f.name)}
                      </span>
                    </span>
                  )}
                  <span className="text-[#6e6e6e] tabular-nums">{formatModified(f.meta_modified_at ?? f.updated_at)}</span>
                  <span className="truncate text-[#6e6e6e]">{fileTypeLabel(f)}</span>
                  <span className="text-[#6e6e6e] tabular-nums text-right">{formatBytes(f.size_bytes)}</span>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      void onFavorite(f);
                    }}
                    className="text-amber-400"
                  >
                    {f.is_favorite ? "★" : "☆"}
                  </button>
                </div>
              );
            })}
          </div>
        )}
        </div>

        <PreviewDrawer
          file={previewFile}
          folderFiles={files}
          folderPathLabel={pathLabel}
          onFileChange={setPreview}
          onClose={() => setPreview(null)}
          actions={{
            onRename: (f) => {
              setPreview(null);
              startRename([{ kind: "file", id: f.id }]);
            },
            onDelete: (f) => {
              const idx = files.findIndex((x) => x.id === f.id);
              const next = idx >= 0 && idx < files.length - 1 ? files[idx + 1] : idx > 0 ? files[idx - 1] : null;
              void onDelete([{ kind: "file", id: f.id }]).then(() => {
                setPreview(next);
              });
            },
            onFavorite: (f) => void onFavorite(f),
          }}
        />
      </div>

      <div className="explorer-statusbar">
        <span className="explorer-statusbar-primary">
          {folders.length + files.length} item{folders.length + files.length === 1 ? "" : "s"}
        </span>
        {selected.length > 0 ? (
          <span className="explorer-statusbar-selection">
            {selected.length} selected
            {selectionStats.bytes > 0 ? ` · ${formatBytes(selectionStats.bytes)}` : null}
          </span>
        ) : null}
        {singleSel && !preview ? (
          <span className="explorer-statusbar-hint">
            {singleSel.kind === "folder" ? "Enter to open" : "Enter or Space to preview · Alt+Enter for properties"}
          </span>
        ) : null}
        {hasClipboardItems() ? (
          <span className="explorer-statusbar-clip">
            {getClipboard()?.mode === "copy" ? "Ready to paste (copy)" : "Ready to paste (move)"}
          </span>
        ) : null}
        {downloadBusy ? <span className="explorer-statusbar-hint">Preparing download…</span> : null}
      </div>
    </>
  );
}
