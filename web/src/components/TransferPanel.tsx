import { useTransfers } from "@/state/transfersContext";

function fmt(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

export function TransferPanel() {
  const { items, cancel, clearDone, retry } = useTransfers();
  if (items.length === 0) return null;

  return (
    <aside className="fixed bottom-4 right-4 z-50 w-full max-w-md rounded border border-[#3d3d3d] bg-[#2d2d2d] p-4 shadow-xl">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold tracking-wide text-zinc-200">Transfers</h2>
        <button
          type="button"
          className="text-xs text-zinc-400 hover:text-zinc-200"
          onClick={clearDone}
        >
          Clear finished
        </button>
      </div>
      <ul className="max-h-64 space-y-3 overflow-y-auto text-sm">
        {items.map((t) => {
          const pct =
            t.totalBytes > 0 ? Math.min(100, Math.round((t.doneBytes / t.totalBytes) * 100)) : 0;
          return (
            <li key={t.id} className="rounded-lg bg-zinc-800/80 p-2">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-zinc-100">{t.name}</p>
                  <p className="text-xs text-zinc-400">
                    {t.status} · {fmt(t.doneBytes)} / {fmt(t.totalBytes)}
                  </p>
                  {t.error ? <p className="text-xs text-red-400">{t.error}</p> : null}
                </div>
                <div className="flex shrink-0 gap-2">
                  {t.status === "error" ? (
                    <button type="button" className="text-xs text-emerald-400 hover:text-emerald-300" onClick={() => retry(t.id)}>
                      Retry
                    </button>
                  ) : null}
                  {(t.status === "queued" || t.status === "running") && (
                    <button type="button" className="text-xs text-zinc-400 hover:text-red-300" onClick={() => cancel(t.id)}>
                      Cancel
                    </button>
                  )}
                </div>
              </div>
              <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-zinc-700">
                <div
                  className="h-full rounded-full bg-emerald-500 transition-all"
                  style={{ width: `${pct}%` }}
                />
              </div>
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
