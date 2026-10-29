import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { getFolder, getFolderChildren } from "@/api/files";
import { ExplorerFolderIcon } from "@/components/ExplorerFolderIcon";

type Props = {
  rootFolderId: string;
  excludeFolderId?: string;
  title?: string;
  onCancel: () => void;
  onSelect: (destinationFolderId: string) => void;
  error?: string | null;
};

export function MoveToDialog({
  rootFolderId,
  excludeFolderId,
  title = "Move to folder",
  onCancel,
  onSelect,
  error,
}: Props) {
  const [browseId, setBrowseId] = useState(rootFolderId);

  const meta = useQuery({
    queryKey: ["folder", browseId],
    queryFn: () => getFolder(browseId),
    enabled: Boolean(browseId),
  });

  const children = useQuery({
    queryKey: ["folder", "children", browseId],
    queryFn: () => getFolderChildren(browseId),
    enabled: Boolean(browseId),
  });

  const crumbs = meta.data?.breadcrumb ?? [];
  const parentId = crumbs.length >= 2 ? crumbs[crumbs.length - 2]?.id : null;
  const folders = (children.data?.folders ?? []).filter((f) => f.id !== excludeFolderId);
  const canChooseHere = browseId !== excludeFolderId;

  return (
    <div className="explorer-modal-backdrop" role="presentation" onClick={onCancel}>
      <div className="explorer-modal explorer-modal-wide" role="dialog" onClick={(e) => e.stopPropagation()}>
        <h2 className="explorer-modal-title">{title}</h2>
        <div className="explorer-move-path">
          {crumbs.map((c, i) => (
            <span key={c.id}>
              {i > 0 ? <span className="text-[#6e6e6e]"> › </span> : null}
              <button type="button" className="explorer-link-btn" onClick={() => setBrowseId(c.id)}>
                {c.name || "Root"}
              </button>
            </span>
          ))}
          {meta.data?.folder?.parent_id ? (
            <>
              <span className="text-[#6e6e6e]"> › </span>
              <span>{meta.data.folder.name}</span>
            </>
          ) : null}
        </div>
        <div className="explorer-move-list">
          {parentId ? (
            <button type="button" className="explorer-move-row explorer-move-row--icon" onClick={() => setBrowseId(parentId)}>
              <ExplorerFolderIcon variant="inline" /> ..
            </button>
          ) : null}
          {children.isLoading ? <p className="p-2 text-xs text-[#9a9a9a]">Loading…</p> : null}
          {folders.map((f) => (
            <button
              key={f.id}
              type="button"
              className="explorer-move-row explorer-move-row--icon"
              onClick={() => setBrowseId(f.id)}
            >
              <ExplorerFolderIcon variant="inline" /> {f.name}
            </button>
          ))}
          {!children.isLoading && folders.length === 0 && !parentId ? (
            <p className="p-2 text-xs text-[#6e6e6e]">No subfolders.</p>
          ) : null}
        </div>
        {error ? <p className="mt-2 text-xs text-red-400">{error}</p> : null}
        <div className="explorer-modal-actions">
          <button type="button" className="explorer-btn" onClick={onCancel}>
            Cancel
          </button>
          <button
            type="button"
            className="explorer-btn explorer-btn-primary"
            disabled={!canChooseHere}
            onClick={() => onSelect(browseId)}
          >
            Move here
          </button>
        </div>
      </div>
    </div>
  );
}
