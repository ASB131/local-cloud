import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
} from "react";
import { ApiError } from "@/api/client";
import { runUploadJob, uploadFile, type UploadOptions } from "@/upload/pipeline";
import { filesFromInputList, walkFileTree, type LocalFileEntry } from "@/upload/directoryWalk";
import {
  loadPersistedTransfers,
  persistTransfers,
  transferReducer,
  type ConflictDetails,
  type TransferItem,
} from "./transfers";

type UploadJob = {
  folderId: string;
  entry: LocalFileEntry;
  uploadName?: string;
  conflictPolicy?: string;
  overwriteFileId?: string;
};

type TransfersState = {
  items: TransferItem[];
  enqueueFiles: (folderId: string, entries: LocalFileEntry[]) => void;
  enqueueFromFileList: (folderId: string, list: FileList) => void;
  enqueueFromDataTransfer: (folderId: string, dt: DataTransfer) => Promise<void>;
  cancel: (id: string) => void;
  clearDone: () => void;
  retry: (id: string) => void;
  resolveConflict: (id: string, action: "overwrite" | "cancel" | "rename", renameTo?: string) => void;
};

const TransfersContext = createContext<TransfersState | null>(null);

function newId() {
  return crypto.randomUUID();
}

export function TransfersProvider({ children }: { children: ReactNode }) {
  const [items, dispatch] = useReducer(transferReducer, [], () => {
    const loaded = loadPersistedTransfers();
    return Array.isArray(loaded) ? loaded : [];
  });
  const controllersRef = useRef(new Map<string, AbortController>());
  const jobsRef = useRef(new Map<string, UploadJob>());

  useEffect(() => {
    persistTransfers(items);
  }, [items]);

  const runJob = useCallback((id: string, job: UploadJob) => {
    const ac = new AbortController();
    controllersRef.current.set(id, ac);
    jobsRef.current.set(id, job);

    const exec = async () => {
      dispatch({ type: "patch", id, patch: { status: "running", error: undefined, conflict: undefined } });
      try {
        await runUploadJob(() =>
          uploadFile({
            transferId: id,
            folderId: job.folderId,
            file: job.entry.file,
            relativePath: job.entry.relativePath,
            uploadName: job.uploadName,
            conflictPolicy: job.conflictPolicy,
            overwriteFileId: job.overwriteFileId,
            signal: ac.signal,
            onProgress: ({ doneBytes }) => {
              dispatch({ type: "patch", id, patch: { doneBytes } });
            },
          } satisfies UploadOptions),
        );
        dispatch({
          type: "patch",
          id,
          patch: { status: "done", doneBytes: job.entry.file.size },
        });
        jobsRef.current.delete(id);
      } catch (e) {
        if (ac.signal.aborted) {
          dispatch({ type: "patch", id, patch: { status: "cancelled" } });
          jobsRef.current.delete(id);
          return;
        }
        if (e instanceof ApiError && e.code === "CONFLICT") {
          const details = (e.details ?? {}) as Record<string, unknown>;
          const conflict: ConflictDetails = {
            file_id: String(details.file_id ?? ""),
            size_bytes: details.size_bytes as number | undefined,
            updated_at: details.updated_at as string | undefined,
          };
          dispatch({ type: "patch", id, patch: { status: "conflict", conflict } });
          return;
        }
        dispatch({
          type: "patch",
          id,
          patch: { status: "error", error: e instanceof Error ? e.message : "Upload failed" },
        });
      } finally {
        controllersRef.current.delete(id);
      }
    };
    void exec();
  }, []);

  const startUpload = useCallback(
    (folderId: string, entry: LocalFileEntry, extra?: Partial<UploadJob>) => {
      const id = newId();
      const job: UploadJob = {
        folderId,
        entry,
        uploadName: extra?.uploadName,
        conflictPolicy: extra?.conflictPolicy,
        overwriteFileId: extra?.overwriteFileId,
      };
      const item: TransferItem = {
        id,
        kind: "upload",
        name: entry.relativePath,
        folderId,
        totalBytes: entry.file.size,
        doneBytes: 0,
        status: "queued",
      };
      dispatch({ type: "add", item });
      runJob(id, job);
    },
    [runJob],
  );

  const enqueueFiles = useCallback(
    (folderId: string, entries: LocalFileEntry[]) => {
      for (const entry of entries) startUpload(folderId, entry);
    },
    [startUpload],
  );

  const enqueueFromFileList = useCallback(
    (folderId: string, list: FileList) => {
      enqueueFiles(folderId, filesFromInputList(list));
    },
    [enqueueFiles],
  );

  const enqueueFromDataTransfer = useCallback(
    async (folderId: string, dt: DataTransfer) => {
      const entries: LocalFileEntry[] = [];
      const list = dt.items;
      if (list && list.length > 0) {
        for (const item of list) {
          const entry = item.webkitGetAsEntry?.();
          if (entry) {
            entries.push(...(await walkFileTree(entry)));
          } else {
            const f = item.getAsFile();
            if (f) entries.push({ file: f, relativePath: f.name });
          }
        }
      } else if (dt.files.length > 0) {
        entries.push(...filesFromInputList(dt.files));
      }
      enqueueFiles(folderId, entries);
    },
    [enqueueFiles],
  );

  const cancel = useCallback((id: string) => {
    controllersRef.current.get(id)?.abort();
    dispatch({ type: "cancel", id });
    jobsRef.current.delete(id);
  }, []);

  const retry = useCallback(
    (id: string) => {
      const job = jobsRef.current.get(id);
      if (!job) return;
      runJob(id, job);
    },
    [runJob],
  );

  const resolveConflict = useCallback(
    (id: string, action: "overwrite" | "cancel" | "rename", renameTo?: string) => {
      const job = jobsRef.current.get(id);
      if (!job) return;
      if (action === "cancel") {
        dispatch({ type: "patch", id, patch: { status: "cancelled" } });
        jobsRef.current.delete(id);
        return;
      }
      const item = items.find((t) => t.id === id);
      const overwriteId = item?.conflict?.file_id;
      let next: UploadJob = { ...job };
      if (action === "rename" && renameTo) {
        next = { ...job, uploadName: renameTo, conflictPolicy: undefined, overwriteFileId: undefined };
      } else if (action === "overwrite") {
        next = { ...job, conflictPolicy: "overwrite", overwriteFileId: overwriteId };
      }
      runJob(id, next);
    },
    [items, runJob],
  );

  const clearDone = useCallback(() => {
    for (const t of items) {
      if (t.status === "done" || t.status === "cancelled" || t.status === "error" || t.status === "conflict") {
        dispatch({ type: "remove", id: t.id });
        jobsRef.current.delete(t.id);
      }
    }
  }, [items]);

  const value = useMemo(
    () => ({
      items,
      enqueueFiles,
      enqueueFromFileList,
      enqueueFromDataTransfer,
      cancel,
      clearDone,
      retry,
      resolveConflict,
    }),
    [items, enqueueFiles, enqueueFromFileList, enqueueFromDataTransfer, cancel, clearDone, retry, resolveConflict],
  );

  return <TransfersContext.Provider value={value}>{children}</TransfersContext.Provider>;
}

export function useTransfers() {
  const ctx = useContext(TransfersContext);
  if (!ctx) throw new Error("TransfersProvider missing");
  return ctx;
}
