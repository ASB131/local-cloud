CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT false,
    is_disabled BOOLEAN NOT NULL DEFAULT false,
    storage_quota_bytes BIGINT NOT NULL CHECK (storage_quota_bytes >= 0),
    storage_used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (storage_used_bytes >= 0),
    speed_limit_kbps INTEGER NULL,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ NULL
);

CREATE TABLE settings (
    id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    global_speed_limit_kbps INTEGER NULL,
    default_user_quota_bytes BIGINT NOT NULL DEFAULT 107374182400,
    trash_retention_days INTEGER NOT NULL DEFAULT 30,
    version_retention_days INTEGER NOT NULL DEFAULT 7,
    version_max_bytes BIGINT NOT NULL DEFAULT 104857600,
    log_retention_days INTEGER NOT NULL DEFAULT 30,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO settings (id) VALUES (true);

CREATE TABLE folders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id UUID NULL REFERENCES folders (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    name_key TEXT NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ NULL,
    deleted_batch_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX folders_one_root_per_user ON folders (user_id) WHERE parent_id IS NULL;

CREATE UNIQUE INDEX folders_sibling_names_active ON folders (user_id, parent_id, name_key)
    WHERE is_deleted = false AND parent_id IS NOT NULL;

CREATE INDEX folders_user_parent_active ON folders (user_id, parent_id) WHERE is_deleted = false;

CREATE TABLE files (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    folder_id UUID NOT NULL REFERENCES folders (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    name_key TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    disk_path TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ NULL,
    deleted_batch_id UUID NULL,
    meta_created_at TIMESTAMPTZ NULL,
    meta_modified_at TIMESTAMPTZ NULL,
    thumbnail_path TEXT NULL,
    thumbnail_status TEXT NOT NULL DEFAULT 'none',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX files_sibling_names_active ON files (folder_id, name_key) WHERE is_deleted = false;

CREATE INDEX files_user_folder_active ON files (user_id, folder_id) WHERE is_deleted = false;

CREATE INDEX files_user_trash ON files (user_id, deleted_at) WHERE is_deleted = true;

CREATE TABLE file_versions (
    id UUID PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    version_num INTEGER NOT NULL,
    size_bytes BIGINT NOT NULL,
    disk_path TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, version_num)
);

CREATE INDEX file_versions_file_version_desc ON file_versions (file_id, version_num DESC);

CREATE INDEX file_versions_created_at ON file_versions (created_at);

CREATE TABLE favorites (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_id UUID NULL REFERENCES files (id) ON DELETE CASCADE,
    folder_id UUID NULL REFERENCES folders (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (file_id IS NOT NULL AND folder_id IS NULL)
        OR (file_id IS NULL AND folder_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX favorites_user_file ON favorites (user_id, file_id) WHERE file_id IS NOT NULL;

CREATE UNIQUE INDEX favorites_user_folder ON favorites (user_id, folder_id) WHERE folder_id IS NOT NULL;

CREATE TABLE invite_codes (
    id UUID PRIMARY KEY,
    code_hash TEXT NOT NULL UNIQUE,
    code_hint TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    is_used BOOLEAN NOT NULL DEFAULT false,
    used_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    default_quota_bytes BIGINT NULL,
    used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE system_logs (
    id UUID PRIMARY KEY,
    user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    message TEXT NOT NULL,
    ip_address INET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX system_logs_created_at ON system_logs (created_at);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    device_label TEXT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_id ON refresh_tokens (user_id);

CREATE TABLE upload_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    folder_id UUID NOT NULL REFERENCES folders (id) ON DELETE CASCADE,
    file_id UUID NULL REFERENCES files (id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    expected_size_bytes BIGINT NULL,
    received_bytes BIGINT NOT NULL DEFAULT 0,
    temp_path TEXT NOT NULL,
    mime_hint TEXT NULL,
    meta_created_at TIMESTAMPTZ NULL,
    meta_modified_at TIMESTAMPTZ NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX upload_sessions_status_expires ON upload_sessions (status, expires_at);

CREATE TABLE thumbnail_jobs (
    id UUID PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX thumbnail_jobs_status_available ON thumbnail_jobs (status, available_at);
