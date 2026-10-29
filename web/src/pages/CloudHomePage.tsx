import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getMe } from "@/api/auth";
import {
  getFile,
  getFolder,
  getFolderChildren,
  listFavorites,
  listFileCategories,
  listTrash,
} from "@/api/files";
import type { CloudFile } from "@/api/types";
import { formatBytes, formatUsedQuota } from "@/lib/formatBytes";
import { ExplorerFileIcon } from "@/components/ExplorerFileIcon";
import { ExplorerFolderIcon } from "@/components/ExplorerFolderIcon";
import { fileCategoryIcon } from "@/lib/fileCategories";
import { fileDisplayName } from "@/lib/fileDisplayName";
import { fileTypeLabel } from "@/lib/fileIcon";
import { loadRecentFiles } from "@/state/explorerRecentFiles";
import { loadRecentFolders } from "@/state/explorerRecent";
import { loadPinnedFolders } from "@/state/explorerPinned";
import { useAuth } from "@/state/auth";
import { useTransfers } from "@/state/transfersContext";

function formatModified(iso: string | null | undefined) {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? "—"
    : d.toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" });
}

export function CloudHomePage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const rootId = user?.root_folder_id ?? "";
  const [searchDraft, setSearchDraft] = useState("");
  const [recentFiles, setRecentFiles] = useState(() => loadRecentFiles());
  const [recentFolders, setRecentFolders] = useState(() => loadRecentFolders());
  const [pinnedFolders, setPinnedFolders] = useState(() => loadPinnedFolders());

  useEffect(() => {
    const tick = () => {
      setRecentFiles(loadRecentFiles());
      setRecentFolders(loadRecentFolders());
      setPinnedFolders(loadPinnedFolders());
    };
    tick();
    const id = window.setInterval(tick, 3000);
    return () => window.clearInterval(id);
  }, []);

  const me = useQuery({ queryKey: ["me"], queryFn: getMe, staleTime: 60_000 });
  const profile = me.data ?? user;

  const favs = useQuery({ queryKey: ["favorites"], queryFn: listFavorites });
  const trash = useQuery({ queryKey: ["trash"], queryFn: listTrash });
  const rootChildren = useQuery({
    queryKey: ["folder", "children", rootId],
    queryFn: () => getFolderChildren(rootId),
    enabled: Boolean(rootId),
  });

  const fileCategories = useQuery({
    queryKey: ["file-categories"],
    queryFn: listFileCategories,
  });

  const favResolved = useQuery({
    queryKey: ["home", "favorites", favs.data?.map((f) => f.id).join(",")],
    enabled: Boolean(favs.data?.length),
    queryFn: async () => {
      const slice = (favs.data ?? []).slice(0, 8);
      return Promise.all(
        slice.map(async (f) => {
          if (f.file_id) {
            const file = await getFile(f.file_id);
            return {
              kind: "file" as const,
              id: file.id,
              folderId: file.folder_id,
              name: file.name,
              mime_type: file.mime_type,
            };
          }
          if (f.folder_id) {
            const { folder } = await getFolder(f.folder_id);
            return { kind: "folder" as const, id: folder.id, name: folder.name || "Folder" };
          }
          return null;
        }),
      );
    },
  });

  const { items: transfers } = useTransfers();
  const activeTransfers = transfers.filter(
    (t) => t.status === "running" || t.status === "queued" || t.status === "conflict",
  );
  const failedTransfers = transfers.filter((t) => t.status === "error").slice(0, 3);

  const used = profile?.storage_used_bytes ?? 0;
  const quota = profile?.storage_quota_bytes ?? 1;
  const pct = quota > 0 ? Math.min(100, (used / quota) * 100) : 0;
  const remaining = Math.max(0, quota - used);

  const rootFolders = rootChildren.data?.folders.length ?? 0;
  const rootFiles = rootChildren.data?.files.length ?? 0;

  const modifiedInRoot = useMemo(() => {
    const list = [...(rootChildren.data?.files ?? [])] as CloudFile[];
    list.sort((a, b) => {
      const ta = new Date(a.meta_modified_at ?? a.updated_at).getTime();
      const tb = new Date(b.meta_modified_at ?? b.updated_at).getTime();
      return tb - ta;
    });
    return list.slice(0, 8);
  }, [rootChildren.data?.files]);

  const favRows = (favResolved.data ?? []).filter(Boolean);

  const onSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const q = searchDraft.trim();
    if (!q || !rootId) return;
    navigate(`/cloud/search?q=${encodeURIComponent(q)}`);
  };

  const openRecentFile = (file: { id: string; folderId: string }) => {
    navigate(`/cloud/folder/${file.folderId}?preview=${encodeURIComponent(file.id)}`);
  };

  return (
    <>
      <div className="explorer-titlebar explorer-titlebar--home">
        <form className="explorer-home-search-form" onSubmit={onSearch}>
          <input
            type="search"
            className="explorer-home-search"
            placeholder="Search all files…"
            value={searchDraft}
            onChange={(e) => setSearchDraft(e.target.value)}
            aria-label="Search files"
          />
          <button type="submit" className="explorer-btn explorer-btn-primary" disabled={!searchDraft.trim() || !rootId}>
            Search
          </button>
        </form>
      </div>
      <div className="explorer-content explorer-home">
        <header className="explorer-home-hero">
          <h1 className="explorer-home-title">Welcome back, {profile?.username ?? "…"}</h1>
          <p className="explorer-home-subtitle">Your home network file hub — pick up where you left off.</p>
        </header>

        <div className="explorer-home-stats">
          <div className="explorer-home-stat">
            <span className="explorer-home-stat-value">{rootFolders + rootFiles}</span>
            <span className="explorer-home-stat-label">Items in My files (top level)</span>
          </div>
          <div className="explorer-home-stat">
            <span className="explorer-home-stat-value">{favs.data?.length ?? "—"}</span>
            <span className="explorer-home-stat-label">Favorites</span>
          </div>
          <div className="explorer-home-stat">
            <span className="explorer-home-stat-value">{trash.data?.length ?? "—"}</span>
            <span className="explorer-home-stat-label">In Recycle Bin</span>
          </div>
          <div className="explorer-home-stat">
            <span className="explorer-home-stat-value">{activeTransfers.length}</span>
            <span className="explorer-home-stat-label">Active transfers</span>
          </div>
        </div>

        <div className="explorer-home-layout">
          <div className="explorer-home-main">
            <section className="explorer-home-card" aria-labelledby="home-quick-heading">
              <h2 id="home-quick-heading" className="explorer-home-card-title">
                Quick access
              </h2>
              <div className="explorer-home-tiles">
                <Link to={rootId ? `/cloud/folder/${rootId}` : "/cloud"} className="explorer-home-tile">
                  <span className="explorer-home-tile-icon" aria-hidden>
                    📂
                  </span>
                  <span className="explorer-home-tile-label">My files</span>
                  <span className="explorer-home-tile-hint">
                    {rootChildren.isLoading ? "…" : `${rootFolders} folders · ${rootFiles} files`}
                  </span>
                </Link>
                <Link to="/cloud/favorites" className="explorer-home-tile">
                  <span className="explorer-home-tile-icon" aria-hidden>
                    ⭐
                  </span>
                  <span className="explorer-home-tile-label">Favorites</span>
                  <span className="explorer-home-tile-hint">Pinned files and folders</span>
                </Link>
                <Link to="/cloud/trash" className="explorer-home-tile">
                  <span className="explorer-home-tile-icon" aria-hidden>
                    🗑️
                  </span>
                  <span className="explorer-home-tile-label">Recycle Bin</span>
                  <span className="explorer-home-tile-hint">
                    {(trash.data?.length ?? 0) > 0 ? `${trash.data!.length} items to review` : "Empty"}
                  </span>
                </Link>
                <Link to="/settings" className="explorer-home-tile">
                  <span className="explorer-home-tile-icon" aria-hidden>
                    👤
                  </span>
                  <span className="explorer-home-tile-label">Account</span>
                  <span className="explorer-home-tile-hint">Settings and profile</span>
                </Link>
                {profile?.is_admin ? (
                  <Link to="/admin" className="explorer-home-tile">
                    <span className="explorer-home-tile-icon" aria-hidden>
                      ⚙️
                    </span>
                    <span className="explorer-home-tile-label">Admin</span>
                    <span className="explorer-home-tile-hint">Users, invites, and logs</span>
                  </Link>
                ) : null}
              </div>
            </section>

            {pinnedFolders.length > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-pinned-heading">
                <h2 id="home-pinned-heading" className="explorer-home-card-title">
                  Pinned folders
                </h2>
                <ul className="explorer-home-pin-list">
                  {pinnedFolders.map((f) => (
                    <li key={f.id}>
                      <Link to={`/cloud/folder/${f.id}`} className="explorer-home-pin-link">
                        <span aria-hidden>📌</span> {f.name}
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            {(fileCategories.data?.length ?? 0) > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-types-heading">
                <h2 id="home-types-heading" className="explorer-home-card-title">
                  Browse by type
                </h2>
                <p className="explorer-home-types-hint">Virtual folders of every file in your cloud, grouped by kind.</p>
                <div className="explorer-home-type-grid">
                  {fileCategories.data!.map((cat) => (
                    <Link
                      key={cat.id}
                      to={`/cloud/folder/${rootId}?category=${encodeURIComponent(cat.id)}`}
                      className="explorer-home-type-folder"
                    >
                      <span className="explorer-home-type-folder-icon" aria-hidden>
                        {fileCategoryIcon(cat.id)}
                      </span>
                      <span className="explorer-home-type-folder-label">{cat.label}</span>
                      <span className="explorer-home-type-folder-count">
                        {cat.count} file{cat.count === 1 ? "" : "s"}
                      </span>
                    </Link>
                  ))}
                </div>
              </section>
            ) : null}

            {modifiedInRoot.length > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-modified-heading">
                <div className="explorer-home-card-head">
                  <h2 id="home-modified-heading" className="explorer-home-card-title explorer-home-card-title--inline">
                    Recently modified in My files
                  </h2>
                  {rootId ? (
                    <Link to={`/cloud/folder/${rootId}`} className="explorer-home-link">
                      Open folder
                    </Link>
                  ) : null}
                </div>
                <ul className="explorer-home-file-table">
                  {modifiedInRoot.map((f) => (
                    <li key={f.id}>
                      <button type="button" className="explorer-home-file-row" onDoubleClick={() => openRecentFile(f)}>
                        <span className="explorer-home-file-name" title={f.name}>
                          {fileDisplayName(f.name)}
                        </span>
                        <span className="explorer-home-file-meta">{fileTypeLabel(f)}</span>
                        <span className="explorer-home-file-meta tabular-nums">{formatBytes(f.size_bytes)}</span>
                        <span className="explorer-home-file-meta">{formatModified(f.meta_modified_at ?? f.updated_at)}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            {recentFiles.length > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-recent-files-heading">
                <h2 id="home-recent-files-heading" className="explorer-home-card-title">
                  Recent files
                </h2>
                <ul className="explorer-home-recent-list">
                  {recentFiles.map((f) => (
                    <li key={f.id}>
                      <button type="button" className="explorer-home-recent-link" onDoubleClick={() => openRecentFile(f)}>
                        <ExplorerFileIcon file={f} variant="inline" />
                        <span className="explorer-home-recent-text" title={f.name}>
                          {fileDisplayName(f.name)}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
          </div>

          <aside className="explorer-home-sidebar">
            <section className="explorer-home-card" aria-labelledby="home-storage-heading">
              <h2 id="home-storage-heading" className="explorer-home-card-title">
                Storage
              </h2>
              <div
                className={`explorer-home-quota-bar${pct >= 90 ? " explorer-home-quota-bar--warn" : ""}`}
                role="progressbar"
                aria-valuenow={Math.round(pct)}
                aria-valuemin={0}
                aria-valuemax={100}
              >
                <div className="explorer-home-quota-fill" style={{ width: `${pct}%` }} />
              </div>
              <p className="explorer-home-quota-label">{formatUsedQuota(used, quota)} used</p>
              <p className="explorer-home-quota-sub">{formatBytes(remaining)} free</p>
              {profile?.effective_speed_limit_kbps ? (
                <p className="explorer-home-quota-sub">
                  Upload speed cap: {profile.effective_speed_limit_kbps} KB/s
                </p>
              ) : null}
            </section>

            {activeTransfers.length > 0 || failedTransfers.length > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-transfers-heading">
                <h2 id="home-transfers-heading" className="explorer-home-card-title">
                  Transfers
                </h2>
                {activeTransfers.length > 0 ? (
                  <ul className="explorer-home-mini-list">
                    {activeTransfers.slice(0, 5).map((t) => (
                      <li key={t.id}>
                        <span className="truncate">{t.name}</span>
                        <span className="explorer-home-mini-meta">{t.status === "conflict" ? "Needs action" : "Uploading…"}</span>
                      </li>
                    ))}
                  </ul>
                ) : null}
                {failedTransfers.length > 0 ? (
                  <p className="explorer-home-warn">{failedTransfers.length} recent upload(s) failed — check the transfer panel.</p>
                ) : null}
              </section>
            ) : null}

            {favRows.length > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-fav-heading">
                <div className="explorer-home-card-head">
                  <h2 id="home-fav-heading" className="explorer-home-card-title explorer-home-card-title--inline">
                    Favorites
                  </h2>
                  <Link to="/cloud/favorites" className="explorer-home-link">
                    See all
                  </Link>
                </div>
                <ul className="explorer-home-recent-list">
                  {favRows.map((row) =>
                    row ? (
                      <li key={`${row.kind}-${row.id}`}>
                        {row.kind === "folder" ? (
                          <Link to={`/cloud/folder/${row.id}`} className="explorer-home-recent-link">
                            <ExplorerFolderIcon variant="inline" />
                            <span className="explorer-home-recent-text">{row.name}</span>
                          </Link>
                        ) : (
                          <Link
                            to={`/cloud/favorites?preview=${encodeURIComponent(row.id)}`}
                            className="explorer-home-recent-link"
                          >
                            <ExplorerFileIcon
                              file={{ id: row.id, name: row.name, mime_type: row.mime_type ?? "" } as CloudFile}
                              variant="inline"
                            />
                            <span className="explorer-home-recent-text" title={row.name}>
                              {fileDisplayName(row.name)}
                            </span>
                          </Link>
                        )}
                      </li>
                    ) : null,
                  )}
                </ul>
              </section>
            ) : null}

            {recentFolders.length > 0 ? (
              <section className="explorer-home-card" aria-labelledby="home-recent-heading">
                <h2 id="home-recent-heading" className="explorer-home-card-title">
                  Recent folders
                </h2>
                <ul className="explorer-home-recent-list">
                  {recentFolders.map((f) => (
                    <li key={f.id}>
                      <Link to={`/cloud/folder/${f.id}`} className="explorer-home-recent-link">
                        <span aria-hidden>📂</span>
                        <span className="explorer-home-recent-text">{f.name}</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            {(trash.data?.length ?? 0) > 0 ? (
              <section className="explorer-home-card explorer-home-card--muted" aria-labelledby="home-trash-heading">
                <h2 id="home-trash-heading" className="explorer-home-card-title">
                  Recycle Bin
                </h2>
                <p className="explorer-home-trash-blurb">
                  {trash.data!.length} deleted item{trash.data!.length === 1 ? "" : "s"} — restore or delete permanently.
                </p>
                <Link to="/cloud/trash" className="explorer-btn explorer-btn-primary explorer-home-trash-btn">
                  Open Recycle Bin
                </Link>
              </section>
            ) : null}
          </aside>
        </div>
      </div>
      <div className="explorer-statusbar">
        <span>Local Cloud · Home</span>
        {pct >= 90 ? <span className="explorer-home-status-warn">Storage almost full</span> : null}
      </div>
    </>
  );
}
