import { ApiError, apiUploadChunk } from "@/api/client";
import * as filesApi from "@/api/files";
import { createFolder } from "@/api/files";

const CHUNK_SIZE = 2 * 1024 * 1024;
const MAX_CONCURRENT = 3;

export type UploadProgress = {
  transferId: string;
  doneBytes: number;
  totalBytes: number;
};

export type UploadOptions = {
  transferId: string;
  folderId: string;
  file: File;
  relativePath?: string;
  /** Override the leaf file name sent to the server (rename-on-conflict). */
  uploadName?: string;
  onProgress: (p: UploadProgress) => void;
  signal?: AbortSignal;
  conflictPolicy?: string;
  overwriteFileId?: string;
  ensureFolderPath?: (parentId: string, relativeDir: string) => Promise<string>;
};

async function ensureDefaultFolderPath(parentId: string, relativeDir: string): Promise<string> {
  if (!relativeDir || relativeDir === "." ) return parentId;
  const parts = relativeDir.split("/").filter(Boolean);
  let current = parentId;
  for (const part of parts) {
    const children = await filesApi.getFolderChildren(current);
    const existing = children.folders.find((f) => f.name === part);
    if (existing) {
      current = existing.id;
    } else {
      const created = await createFolder(current, part);
      current = created.id;
    }
  }
  return current;
}

export async function uploadFile(opts: UploadOptions): Promise<void> {
  const rel = opts.relativePath ?? opts.file.name;
  const slash = rel.lastIndexOf("/");
  const fileName = opts.uploadName ?? (slash >= 0 ? rel.slice(slash + 1) : rel);
  const dir = slash >= 0 ? rel.slice(0, slash) : "";
  const ensure = opts.ensureFolderPath ?? ensureDefaultFolderPath;
  const targetFolder = await ensure(opts.folderId, dir);

  const metaModified =
    opts.file.lastModified > 0 ? new Date(opts.file.lastModified).toISOString() : null;

  let session;
  try {
    session = await filesApi.initUpload({
      folder_id: targetFolder,
      name: fileName,
      expected_size_bytes: opts.file.size,
      mime_hint: opts.file.type || undefined,
      meta_modified_at: metaModified,
      conflict_policy: opts.conflictPolicy,
      file_id: opts.overwriteFileId,
    });
  } catch (e) {
    if (e instanceof ApiError && e.code === "CONFLICT") {
      throw e;
    }
    throw e;
  }

  let offset = session.received_bytes;
  opts.onProgress({ transferId: opts.transferId, doneBytes: offset, totalBytes: opts.file.size });

  while (offset < opts.file.size) {
    if (opts.signal?.aborted) {
      await filesApi.abortUpload(session.id);
      throw new DOMException("Aborted", "AbortError");
    }
    const end = Math.min(offset + CHUNK_SIZE, opts.file.size);
    const chunk = opts.file.slice(offset, end);
    const res = await apiUploadChunk(session.id, offset, chunk, opts.signal);
    offset = res.received_bytes;
    opts.onProgress({ transferId: opts.transferId, doneBytes: offset, totalBytes: opts.file.size });
  }

  await filesApi.completeUpload(session.id);
}

type QueueJob = () => Promise<void>;

let active = 0;
const waitQueue: Array<() => void> = [];

function acquireSlot(): Promise<void> {
  if (active < MAX_CONCURRENT) {
    active += 1;
    return Promise.resolve();
  }
  return new Promise((resolve) => waitQueue.push(resolve));
}

function releaseSlot() {
  active -= 1;
  const next = waitQueue.shift();
  if (next) {
    active += 1;
    next();
  }
}

export async function runUploadJob(job: QueueJob): Promise<void> {
  await acquireSlot();
  try {
    await job();
  } finally {
    releaseSlot();
  }
}
