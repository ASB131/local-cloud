DROP TABLE IF EXISTS office_preview_jobs;
ALTER TABLE files DROP COLUMN IF EXISTS office_preview_path;
ALTER TABLE files DROP COLUMN IF EXISTS office_preview_status;
