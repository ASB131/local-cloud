import type { CloudFile } from "@/api/types";
import { uploadFile } from "@/upload/pipeline";

/** Replace an existing file's bytes in place (creates a new version when policy applies). */
export async function replaceFileContent(file: CloudFile, body: Blob): Promise<void> {
  const upload = new File([body], file.name, {
    type: body.type || file.mime_type || "application/octet-stream",
    lastModified: Date.now(),
  });
  await uploadFile({
    transferId: `save-${file.id}-${Date.now()}`,
    folderId: file.folder_id,
    file: upload,
    uploadName: file.name,
    conflictPolicy: "overwrite",
    overwriteFileId: file.id,
    onProgress: () => {},
  });
}
