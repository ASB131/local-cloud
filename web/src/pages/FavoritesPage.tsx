import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { getFile, getFolder, listFavorites } from "@/api/files";
import type { CloudFile } from "@/api/types";
import { ExplorerFileIcon } from "@/components/ExplorerFileIcon";
import { ExplorerFolderIcon } from "@/components/ExplorerFolderIcon";
import { PreviewDrawer } from "@/components/PreviewDrawer";
import { fileDisplayName } from "@/lib/fileDisplayName";
import { pushRecentFile } from "@/state/explorerRecentFiles";

type FavRow =
  | { kind: "folder"; id: string; name: string; favId: string }
  | { kind: "file"; file: CloudFile; favId: string };

export function FavoritesPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [preview, setPreview] = useState<CloudFile | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  const favs = useQuery({ queryKey: ["favorites"], queryFn: listFavorites });

  const resolved = useQuery({
    queryKey: ["favorites", "resolved", favs.data],
    enabled: Boolean(favs.data?.length),
    queryFn: async () => {
      const items = favs.data ?? [];
      return Promise.all(
        items.map(async (f): Promise<FavRow | null> => {
          if (f.file_id) {
            const file = await getFile(f.file_id);
            return { kind: "file", file, favId: f.id };
          }
          if (f.folder_id) {
            const { folder } = await getFolder(f.folder_id);
            return { kind: "folder", id: folder.id, name: folder.name || "Folder", favId: f.id };
          }
          return null;
        }),
      );
    },
  });

  const rows = (resolved.data ?? []).filter((r): r is FavRow => r != null);
  const files = useMemo(() => rows.filter((r) => r.kind === "file").map((r) => r.file), [rows]);

  const previewFile = useMemo(
    () => (preview ? (files.find((f) => f.id === preview.id) ?? preview) : null),
    [preview, files],
  );

  useEffect(() => {
    const previewId = searchParams.get("preview");
    if (!previewId || resolved.isLoading) return;
    const f = files.find((x) => x.id === previewId);
    if (f) {
      setPreview(f);
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete("preview");
          return next;
        },
        { replace: true },
      );
    }
  }, [files, resolved.isLoading, searchParams, setSearchParams]);

  const openFile = (f: CloudFile) => {
    pushRecentFile({ id: f.id, name: f.name, folderId: f.folder_id });
    setPreview(f);
  };

  return (
    <>
      <div className="explorer-titlebar">
        <div className="explorer-address">
          <Link to="/cloud">Home</Link>
          <span className="text-[#6e6e6e]">›</span>
          <span>Favorites</span>
        </div>
      </div>
      <div className="explorer-content-select-host">
        <div className="explorer-content">
          {favs.isLoading || resolved.isLoading ? <p className="text-[#9a9a9a]">Loading…</p> : null}
          {rows.length === 0 && !favs.isLoading ? (
            <p className="py-12 text-center text-[#6e6e6e]">No favorites yet.</p>
          ) : null}
          <div className="explorer-grid">
            {rows.map((row) =>
              row.kind === "folder" ? (
                <div
                  key={row.favId}
                  role="button"
                  tabIndex={0}
                  className={`explorer-tile ${selectedKey === `folder:${row.id}` ? "selected" : ""}`}
                  onClick={() => setSelectedKey(`folder:${row.id}`)}
                  onDoubleClick={() => navigate(`/cloud/folder/${row.id}`)}
                >
                  <ExplorerFolderIcon variant="grid" />
                  <div className="explorer-tile-name">{row.name}</div>
                </div>
              ) : (
                <div
                  key={row.favId}
                  role="button"
                  tabIndex={0}
                  className={`explorer-tile ${selectedKey === `file:${row.file.id}` ? "selected" : ""}`}
                  onClick={() => setSelectedKey(`file:${row.file.id}`)}
                  onDoubleClick={() => openFile(row.file)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") openFile(row.file);
                  }}
                >
                  <ExplorerFileIcon file={row.file} variant="grid" />
                  <div className="explorer-tile-name" title={row.file.name}>
                    {fileDisplayName(row.file.name)}
                  </div>
                </div>
              ),
            )}
          </div>
        </div>
        <PreviewDrawer
          file={previewFile}
          folderFiles={files}
          folderPathLabel="Favorites"
          onFileChange={setPreview}
          onClose={() => setPreview(null)}
        />
      </div>
      <div className="explorer-statusbar">
        <span>{rows.length} favorite{rows.length === 1 ? "" : "s"}</span>
      </div>
    </>
  );
}
