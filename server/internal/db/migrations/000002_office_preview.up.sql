ALTER TABLE files
    ADD COLUMN office_preview_status TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN office_preview_path TEXT NULL;

CREATE TABLE office_preview_jobs (
    id UUID PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX office_preview_jobs_pending ON office_preview_jobs (status, available_at)
    WHERE status = 'pending';
