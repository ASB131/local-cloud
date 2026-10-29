import { apiDownloadBlob, apiDownloadPost, apiRequest } from "./client";
import type {
  ChildrenResponse,
  Favorite,
  Folder,
  MediaTokenResponse,
  SearchHit,
  TrashItem,
  UploadSession,
  CloudFile,
  FileVersion,
} from "./types";

export function getFolderChildren(folderId: string) {
  return apiRequest<ChildrenResponse>(`/api/v1/folders/${folderId}/children`).then((data) => ({
    folders: data.folders ?? [],
    files: data.files ?? [],
  }));
}

export function getFolder(folderId: string) {
  return apiRequest<{ folder: Folder; breadcrumb: Folder[] }>(`/api/v1/folders/${folderId}`).then(
    (data) => ({
      folder: data.folder,
      breadcrumb: data.breadcrumb ?? [],
    }),
  );
}

export function createFolder(parentId: string, name: string) {
  return apiRequest<Folder>("/api/v1/folders", {
    method: "POST",
    body: { parent_id: parentId, name },
  });
}

export function patchFolder(id: string, patch: { name?: string; parent_id?: string }) {
  return apiRequest<Folder>(`/api/v1/folders/${id}`, { method: "PATCH", body: patch });
}

export function deleteFolder(id: string) {
  return apiRequest<void>(`/api/v1/folders/${id}`, { method: "DELETE" });
}

export function getFile(id: string) {
  return apiRequest<CloudFile>(`/api/v1/files/${id}`);
}

export function patchFile(id: string, patch: { name?: string; folder_id?: string }) {
  return apiRequest<CloudFile>(`/api/v1/files/${id}`, { method: "PATCH", body: patch });
}

export function deleteFile(id: string) {
  return apiRequest<void>(`/api/v1/files/${id}`, { method: "DELETE" });
}

export function copyFile(id: string, folderId?: string) {
  return apiRequest<CloudFile>(`/api/v1/files/${id}/copy`, {
    method: "POST",
    body: folderId ? { folder_id: folderId } : {},
  });
}

export function copyFolder(id: string, parentId?: string) {
  return apiRequest<Folder>(`/api/v1/folders/${id}/copy`, {
    method: "POST",
    body: parentId ? { parent_id: parentId } : {},
  });
}

export function listFileVersions(fileId: string) {
  return apiRequest<{ items: FileVersion[] }>(`/api/v1/files/${fileId}/versions`).then((d) => d.items ?? []);
}

export function restoreFileVersion(fileId: string, versionId: string) {
  return apiRequest<CloudFile>(`/api/v1/files/${fileId}/versions/${versionId}/restore`, { method: "POST", body: {} });
}

export function initUpload(body: {
  folder_id: string;
  name: string;
  expected_size_bytes?: number;
  mime_hint?: string;
  meta_created_at?: string | null;
  meta_modified_at?: string | null;
  conflict_policy?: string;
  file_id?: string;
}) {
  return apiRequest<UploadSession>("/api/v1/uploads", { method: "POST", body });
}

export function completeUpload(sessionId: string) {
  return apiRequest<CloudFile>(`/api/v1/uploads/${sessionId}/complete`, { method: "POST" });
}

export function abortUpload(sessionId: string) {
  return apiRequest<void>(`/api/v1/uploads/${sessionId}`, { method: "DELETE" });
}

export function issueMediaToken(fileId: string) {
  return apiRequest<MediaTokenResponse>(`/api/v1/files/${fileId}/media-token`, { method: "POST" });
}

export function issueVersionMediaToken(fileId: string, versionId: string) {
  return apiRequest<MediaTokenResponse>(`/api/v1/files/${fileId}/versions/${versionId}/media-token`, {
    method: "POST",
  });
}

export type FileCategorySummary = { id: string; label: string; count: number };

export function listFileCategories() {
  return apiRequest<{ items: FileCategorySummary[] }>("/api/v1/file-categories").then((d) => d.items ?? []);
}

export function listFilesByCategory(category: string) {
  return apiRequest<{ items: CloudFile[] }>(`/api/v1/file-categories/${encodeURIComponent(category)}/files`).then(
    (d) => d.items ?? [],
  );
}

export function searchFiles(q: string) {
  return apiRequest<{ items: SearchHit[] }>(`/api/v1/search?q=${encodeURIComponent(q)}`).then(
    (data) => data.items ?? [],
  );
}

export function listTrash() {
  return apiRequest<{ items: TrashItem[] }>("/api/v1/trash").then((d) => d.items ?? []);
}

export function restoreTrashFile(
  id: string,
  body?: {
    destination_folder_id?: string;
    conflict_policy?: string;
    name?: string;
    file_id?: string;
  },
) {
  return apiRequest<CloudFile>(`/api/v1/trash/files/${id}/restore`, { method: "POST", body: body ?? {} });
}

export function restoreTrashFolder(id: string) {
  return apiRequest<void>(`/api/v1/trash/folders/${id}/restore`, { method: "POST", body: {} });
}

export function hardDeleteTrashFile(id: string) {
  return apiRequest<void>(`/api/v1/trash/files/${id}`, { method: "DELETE" });
}

export function hardDeleteTrashFolder(id: string) {
  return apiRequest<void>(`/api/v1/trash/folders/${id}`, { method: "DELETE" });
}

export function listFavorites() {
  return apiRequest<{ items: Favorite[] }>("/api/v1/favorites").then((d) => d.items ?? []);
}

export function addFavorite(body: { file_id?: string; folder_id?: string }) {
  return apiRequest<Favorite>("/api/v1/favorites", { method: "POST", body });
}

export function removeFavorite(id: string) {
  return apiRequest<void>(`/api/v1/favorites/${id}`, { method: "DELETE" });
}

export async function downloadFile(fileId: string, filename: string) {
  const blob = await apiDownloadBlob(`/api/v1/files/${fileId}/content`);
  triggerSave(blob, filename);
}

export function getClientFeatures() {
  return apiRequest<{ onlyoffice_public_url: string; tls_enabled: boolean }>("/api/v1/client/features");
}

export type OnlyOfficeConfig = {
  document_server_url: string;
  document_type: string;
  document: {
    fileType: string;
    key: string;
    title: string;
    url: string;
  };
};

export function getOnlyOfficeConfig(fileId: string) {
  return apiRequest<OnlyOfficeConfig>(`/api/v1/files/${fileId}/onlyoffice-config`);
}

export async function downloadArchive(folderIds: string[], fileIds: string[], zipName = "archive.zip") {
  const blob = await apiDownloadPost("/api/v1/downloads/archive", {
    folder_ids: folderIds,
    file_ids: fileIds,
  });
  triggerSave(blob, zipName);
}

function triggerSave(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}
