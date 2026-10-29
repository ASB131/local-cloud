export type ApiErrorCode =
  | "VALIDATION"
  | "INVALID_CREDENTIALS"
  | "UNAUTHENTICATED"
  | "ACCOUNT_DISABLED"
  | "RATE_LIMITED"
  | "LOCKED"
  | "NOT_FOUND"
  | "CONFLICT"
  | "QUOTA_EXCEEDED"
  | "RANGE_NOT_SATISFIABLE"
  | "INTERNAL"
  | "PARENT_DELETED"
  | "CYCLE";

export type ApiErrorBody = {
  error: {
    code: ApiErrorCode;
    message: string;
    details?: Record<string, unknown>;
  };
};

export type UserProfile = {
  id: string;
  username: string;
  is_admin: boolean;
  storage_quota_bytes: number;
  storage_used_bytes: number;
  speed_limit_kbps: number | null;
  effective_speed_limit_kbps: number | null;
  root_folder_id: string;
};

export type AuthResponse = {
  access_token: string;
  access_expires_at: string;
  refresh_token: string;
  user: UserProfile;
};

export type Folder = {
  id: string;
  parent_id?: string | null;
  name: string;
  created_at: string;
  updated_at: string;
};

export type FileVersion = {
  id: string;
  version_num: number;
  size_bytes: number;
  file_hash: string;
  created_at: string;
};

export type CloudFile = {
  id: string;
  folder_id: string;
  name: string;
  size_bytes: number;
  mime_type: string;
  file_hash: string;
  meta_created_at: string | null;
  meta_modified_at: string | null;
  created_at: string;
  updated_at: string;
  thumbnail_status: string;
  office_preview_status?: string;
  is_favorite: boolean;
};

export type ChildrenResponse = {
  folders: Folder[];
  files: CloudFile[];
};

export type UploadSession = {
  id: string;
  received_bytes: number;
  expires_at: string;
};

export type MediaTokenResponse = {
  token: string;
  expires_at: string;
};

export type CloudWsEvent = {
  type: string;
  payload?: unknown;
};

export type TrashItem = {
  kind: "file" | "folder";
  id: string;
  name: string;
  size_bytes?: number | null;
  deleted_at: string;
};

export type Favorite = {
  id: string;
  file_id?: string | null;
  folder_id?: string | null;
  created_at: string;
};

export type SearchHit = {
  kind: "file" | "folder";
  item: Folder | CloudFile;
};
