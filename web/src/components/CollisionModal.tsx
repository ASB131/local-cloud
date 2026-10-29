import { useState } from "react";
import type { TransferItem } from "@/state/transfers";

type Props = {
  transfer: TransferItem;
  onOverwrite: () => void;
  onRename: (newName: string) => void;
  onCancel: () => void;
};

export function CollisionModal({ transfer, onOverwrite, onRename, onCancel }: Props) {
  const [rename, setRename] = useState("");
  const base = transfer.name.includes("/") ? transfer.name.split("/").pop()! : transfer.name;
  const dot = base.lastIndexOf(".");
  const suggested =
    dot > 0 ? `${base.slice(0, dot)} (1)${base.slice(dot)}` : `${base} (1)`;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="w-full max-w-md rounded-xl border border-zinc-700 bg-zinc-900 p-5 shadow-2xl">
        <h3 className="text-lg font-semibold">File already exists</h3>
        <p className="mt-2 text-sm text-zinc-400">
          <span className="text-zinc-200">{transfer.name}</span> conflicts with an existing file in this folder.
        </p>
        <label className="mt-4 block text-sm">
          Rename to
          <input
            className="mt-1 w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2"
            placeholder={suggested}
            value={rename}
            onChange={(e) => setRename(e.target.value)}
          />
        </label>
        <div className="mt-5 flex flex-wrap justify-end gap-2">
          <button type="button" className="rounded-lg px-3 py-1.5 text-sm text-zinc-400 hover:bg-zinc-800" onClick={onCancel}>
            Cancel
          </button>
          <button
            type="button"
            className="rounded-lg border border-zinc-600 px-3 py-1.5 text-sm hover:bg-zinc-800"
            onClick={() => onRename(rename.trim() || suggested)}
          >
            Upload as new name
          </button>
          <button
            type="button"
            className="rounded-lg bg-amber-600 px-3 py-1.5 text-sm font-medium hover:bg-amber-500"
            onClick={onOverwrite}
          >
            Overwrite
          </button>
        </div>
      </div>
    </div>
  );
}
