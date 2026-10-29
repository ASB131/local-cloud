import { apiRequest } from "./client";

export type AdminUser = {
  id: string;
  username: string;
  is_admin: boolean;
  is_disabled: boolean;
  storage_quota_bytes: number;
  storage_used_bytes: number;
  speed_limit_kbps: number | null;
  created_at?: string;
};

export type InviteListItem = {
  id: string;
  code_hint: string;
  expires_at: string;
  is_used: boolean;
  used_by_username: string | null;
  created_at: string;
};

export type InviteCreateResult = {
  id: string;
  code: string;
  expires_at: string;
};

export type AdminSettings = {
  global_speed_limit_kbps: number | null;
  version_retention_days: number;
  version_max_bytes: number;
};

export type AdminStats = {
  user_count: number;
  total_used_bytes: number;
  total_quota_bytes: number;
  open_upload_sessions: number;
};

export type LogEntry = {
  id: string;
  user_id?: string;
  event_type: string;
  message: string;
  created_at: string;
};

export type RebuildReport = {
  started_at: string;
  finished_at: string;
  files_repaired: number;
  files_inserted?: number;
  files_missing: number;
  orphans_skipped: number;
  errors: number;
  sample_missing_file_ids?: string[];
};

export function listUsers() {
  return apiRequest<{ items: AdminUser[] }>("/api/v1/admin/users");
}

export function patchUser(
  id: string,
  patch: {
    storage_quota_bytes?: number;
    speed_limit_kbps?: number;
    is_admin?: boolean;
    is_disabled?: boolean;
  },
) {
  return apiRequest<void>(`/api/v1/admin/users/${id}`, { method: "PATCH", body: patch });
}

export function deleteUser(id: string) {
  return apiRequest<void>(`/api/v1/admin/users/${id}`, { method: "DELETE" });
}

export function listInvites() {
  return apiRequest<{ items: InviteListItem[] }>("/api/v1/admin/invites");
}

export function createInvite(body?: { expires_at?: string; default_quota_bytes?: number }) {
  return apiRequest<InviteCreateResult>("/api/v1/admin/invites", { method: "POST", body: body ?? {} });
}

export function revokeInvite(id: string) {
  return apiRequest<void>(`/api/v1/admin/invites/${id}`, { method: "DELETE" });
}

export function getSettings() {
  return apiRequest<AdminSettings>("/api/v1/admin/settings");
}

export function patchSettings(globalSpeedLimitKbps: number | null) {
  return apiRequest<void>("/api/v1/admin/settings", {
    method: "PATCH",
    body: { global_speed_limit_kbps: globalSpeedLimitKbps },
  });
}

export function getStats() {
  return apiRequest<AdminStats>("/api/v1/admin/stats");
}

export function getLogs(cursor?: string) {
  const q = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  return apiRequest<{ items: LogEntry[] }>(`/api/v1/admin/logs${q}`);
}

export function startRebuild() {
  return apiRequest<RebuildReport>("/api/v1/admin/rebuild", { method: "POST", body: {} });
}

export function getLastRebuild() {
  return apiRequest<RebuildReport>("/api/v1/admin/rebuild");
}
