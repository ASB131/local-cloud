import { ExplorerFolderIcon } from "@/components/ExplorerFolderIcon";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  hardDeleteTrashFile,
  hardDeleteTrashFolder,
  listTrash,
  restoreTrashFile,
  restoreTrashFolder,
} from "@/api/files";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { fileDisplayName } from "@/lib/fileDisplayName";
import { useAuth } from "@/state/auth";

type TrashItem = { kind: string; id: string; name: string };

export function TrashPage() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const [purgeTarget, setPurgeTarget] = useState<TrashItem | null>(null);
  const trash = useQuery({ queryKey: ["trash"], queryFn: listTrash });

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ["trash"] });
    qc.invalidateQueries({ queryKey: ["folder"] });
  };

  const restoreMut = useMutation({
    mutationFn: async (item: TrashItem) => {
      if (item.kind === "file") {
        try {
          await restoreTrashFile(item.id);
        } catch {
          await restoreTrashFile(item.id, { destination_folder_id: user?.root_folder_id });
        }
      } else {
        await restoreTrashFolder(item.id);
      }
    },
    onSuccess: refresh,
  });

  const purgeMut = useMutation({
    mutationFn: async (item: TrashItem) => {
      if (item.kind === "file") await hardDeleteTrashFile(item.id);
      else await hardDeleteTrashFolder(item.id);
    },
    onSuccess: () => {
      setPurgeTarget(null);
      refresh();
    },
  });

  return (
    <>
      <div className="explorer-titlebar">
        <div className="explorer-address">
          <span>Cloud</span>
          <span className="text-[#6e6e6e]">›</span>
          <span>Recycle Bin</span>
        </div>
      </div>
      <div className="explorer-content">
        <h1 className="mb-4 text-lg font-normal">Recycle Bin</h1>
        {trash.isLoading ? <p className="text-[#9a9a9a]">Loading…</p> : null}
        {(trash.data ?? []).length === 0 && !trash.isLoading ? (
          <p className="py-12 text-center text-[#6e6e6e]">Recycle Bin is empty.</p>
        ) : null}
        <div className="explorer-list">
          {(trash.data ?? []).map((item) => (
            <div key={`${item.kind}-${item.id}`} className="explorer-list-row grid-cols-[1fr_auto_auto]">
              <span className="explorer-list-name">
                {item.kind === "folder" ? (
                  <ExplorerFolderIcon variant="inline" />
                ) : (
                  <img
                    src="/icons/file-types/file.png"
                    alt=""
                    className="explorer-inline-file-type-icon"
                    draggable={false}
                  />
                )}
                {item.kind === "file" ? (
                  <span title={item.name}>{fileDisplayName(item.name)}</span>
                ) : (
                  item.name
                )}
              </span>
              <button type="button" className="explorer-btn" onClick={() => restoreMut.mutate(item)}>
                Restore
              </button>
              <button type="button" className="explorer-btn text-red-300" onClick={() => setPurgeTarget(item)}>
                Delete
              </button>
            </div>
          ))}
        </div>
      </div>
      <div className="explorer-statusbar">
        <span>{(trash.data ?? []).length} item{(trash.data ?? []).length === 1 ? "" : "s"}</span>
      </div>

      {purgeTarget ? (
        <ConfirmDialog
          title="Delete permanently"
          message={`Remove “${purgeTarget.name}” forever? This cannot be undone.`}
          confirmLabel="Delete"
          danger
          onCancel={() => setPurgeTarget(null)}
          onConfirm={() => purgeMut.mutate(purgeTarget)}
        />
      ) : null}
    </>
  );
}
