# Local Cloud — Master Specification and Roadmap

**Status:** Specification updated for **web-only client** (LAN URL). No application code has been written.
**Product:** Local Cloud — enterprise-grade, self-hosted, LAN-only cloud storage.
**Rule:** This document is the source of truth. Later phases must follow it. A change to a settled decision requires a new entry in the Decision Log before code diverges.

---

## 1. Purpose

Local Cloud is a decoupled private cloud. The server owns a per-user folder tree that is **not** a mirror of any client PC’s disk. A person uses a **web browser** on the LAN (for example `http://192.168.1.50:8090/`) to browse, upload, download, preview, and organize remote objects. The server also serves the web UI from the same host and port. There is no separate desktop installer. The browser does not maintain a vault-sized offline cache; uploads and downloads run in the page with a visible transfer list. It is not a continuous 1:1 sync engine (not Dropbox, not Syncthing).

Security boundary for v1: **the home LAN only**. Clients must be on the same local network as the server (Wi‑Fi or Ethernet behind the same router). The product is **not** designed for access from the public internet, from port-forwarding, or from **VPN/tunnel remote access** (Tailscale, WireGuard into the LAN, etc.). The host administrator is trusted. There is no end-to-end encryption in v1; bytes on disk are the original file bytes.

---

## 2. Non-goals (v1)

- Public internet, DNS, Let’s Encrypt, or reverse-proxy TLS termination.
- Remote access of any kind (VPN into the LAN, mesh networks, “phone from away,” cloud relay). Use the service only while your device is on the home LAN.
- Native desktop or mobile apps (Tauri, Electron, store apps). The product is a responsive web app; phone browsers may work but are not a separate deliverable.
- End-to-end encryption, client-side encryption, or chunk-level deduplication.
- Continuous watched-folder sync in either direction.
- Sharing links, public links, or cross-user shares.
- Multi-node storage, object storage (S3), or file chunking.
- Online document editing.

Out of scope stays out of scope until this document is revised.

---

## 3. Architecture

```text
+---------------------------- LAN --------------------------------+
|                                                                  |
|  Any PC / tablet browser                                         |
|  http://<host-lan-ip>:<LOCAL_CLOUD_PORT>/  (default 8090)            |
|  +-----------------------------------------------------------+   |
|  | React + TypeScript + Tailwind SPA (served by localcloud-api)   |   |
|  | cloud browser, previews, admin, in-page transfer list     |   |
|  +---------------------------+-------------------------------+   |
|                              | same origin                      |
|                              v                                  |
|  Docker Compose                                                  |
|  +---------------------------+    +---------------------------+  |
|  | localcloud-api (Go)            |    | PostgreSQL 16             |  |
|  | embedded static UI,       |--->| users, tree, versions,    |  |
|  | REST, WebSocket hub,      |    | logs, jobs                |  |
|  | stream I/O, workers, cron |    |                           |  |
|  +-------------+-------------+    +---------------------------+  |
|                |                                                 |
|                v                                                 |
|  Host volume /data/users/{user_id}/...                           |
|  Un-chunked file bytes + small metadata sidecars                 |
|                                                                  |
|  Watchtower: updates the localcloud-api image only                    |
+------------------------------------------------------------------+
```

### 3.1 Component responsibilities

| Component | Owns | Does not own |
|---|---|---|
| PostgreSQL | Users, invites, folder tree, file metadata, versions, favorites, logs, settings, upload sessions, jobs | File bytes |
| Disk under `/data` | File bytes, thumbnails, per-object `meta.json`, SQL dumps | Authority over names while the database is healthy |
| Go API | Auth, authorization, streaming I/O, quotas, throttling, zip, range reads, WebSocket fan-out, background jobs, **production SPA static files** | Business logic in the browser beyond presentation |
| Web SPA | Presentation, upload/download orchestration, in-memory transfer state, WebSocket handling | Remote tree source of truth, direct disk access |
| Watchtower | Pulling a newer `localcloud-api` image | Postgres image upgrades |

### 3.2 Source of truth

1. While PostgreSQL is healthy, the database is the authority for names, tree structure, quotas, and deletion state.
2. Disk is the authority for bytes. A database row whose blob is missing is an integrity error, not a silent empty file.
3. `meta.json` sidecars exist so an admin can rebuild metadata after database loss. They are written by the server, never trusted from the client.
4. Browser storage (memory, optional `sessionStorage` for session restore) is disposable. Clearing site data logs the user out and drops in-flight UI state; vault data remains on the server.

### 3.3 Trust and transport

- Transport is **HTTP on the LAN**, as required. TLS is not part of v1.
- Consequence, recorded on purpose: bearer tokens and file bytes are visible to anyone who can sniff the LAN segment. Mitigation is network placement (isolated VLAN, host firewall, no port forward), not application crypto.
- The API listens on **`0.0.0.0` and a configurable TCP port** (default **8090**, not 8080 — port 8080 is commonly used by qBittorrent and other home-server apps). Compose publishes that port to the host LAN so users open **`http://<machine-lan-ip>:<port>/`** (or the host’s LAN hostname). The port is set in `deploy/.env` (see §14.2). Postgres is **not** published to the host.
- **Same origin in production:** the UI is served from `/` and `/assets/*`; JSON and streams live under `/api/v1/*`. The SPA uses relative URLs (`/api/v1/...`), so no “server URL” screen is required in production.
- **Development:** Vite dev server on a separate port proxies `/api` to the Go process. CORS is not used in production; in dev the proxy keeps the browser on one origin.
- API calls use `Authorization: Bearer <access_token>` except media elements (see §9.5 and §13.4).
- Refresh tokens are stored in **`sessionStorage`** under a fixed key. Access tokens live in memory only. This is an explicit LAN tradeoff (XSS could exfiltrate refresh tokens). The UI must not log tokens and must not put them in URLs.

---

## 4. Technology stack

### 4.1 Server

| Concern | Choice |
|---|---|
| Language | Go (current stable 1.23+ at implementation time) |
| HTTP router | `chi` |
| Postgres driver | `pgx/v5` |
| Migrations | `golang-migrate` SQL files in `server/migrations` |
| Passwords | Argon2id (OWASP parameters: 64 MiB memory, 3 iterations, 4 parallelism, 16-byte salt, 32-byte key) |
| Access tokens | JWT (EdDSA or HS256 from a long random secret), 15-minute lifetime, claims: `sub`, `username`, `admin`, `exp`, `iat`, `jti` |
| Refresh tokens | 32-byte random, stored as SHA-256, 30-day sliding lifetime, rotated on every refresh, revocable |
| WebSockets | `coder/websocket` |
| Logging | `log/slog` JSON to stdout. Application audit trail is `system_logs`, separate from stdout |
| IDs | UUID v7 (time-ordered) generated in Go |
| Thumbnails | `ffmpeg` in the API image. Still images may be scaled in-process if ffmpeg is unnecessary; video always uses ffmpeg |
| Archive download | Go `archive/zip` streaming writer (`zip.Store` for already-compressed types, `zip.Deflate` otherwise) |
| Config | Environment variables only. No config file inside the image |
| Static UI | Production build embedded in the binary (`embed.FS`) or copied into the image at `/app/web`; served with cache headers for hashed assets |

### 4.2 Data and deployment

| Concern | Choice |
|---|---|
| Database | PostgreSQL 16 |
| App files | Un-chunked on the server disk |
| Orchestration | Docker Compose |
| Images | `postgres:16`, self-built `localcloud-api`, `containrrr/watchtower` |
| Watchtower scope | Label-selected **app container only**. Postgres is upgraded by a human |
| Time | UTC in the database, on disk, and on the wire. The UI renders local time |

### 4.3 Web client

| Concern | Choice |
|---|---|
| Tooling | Vite + React 18 + TypeScript + Tailwind CSS |
| Routing | React Router (or TanStack Router); client-side routes under `/`, `/login`, `/cloud`, `/trash`, `/admin`, etc. |
| Server state | TanStack Query (or equivalent) for folder listings and metadata; WebSocket invalidates or patches cache |
| Transfers | In-app queue (React state + optional `sessionStorage` snapshot for refresh). No IndexedDB vault cache in v1 |
| Uploads | `fetch` + `ReadableStream` from `File` / `Blob`; resume via upload session `Offset` header |
| Downloads | `fetch` stream to disk via browser download (`<a download>` blob URL for small files; `showSaveFilePicker` where supported; fallback anchor download) |
| Auth storage | Access JWT in memory; refresh token in `sessionStorage`; cleared on logout |
| Media preview | Short-lived **media tokens** in query string for `<video>` and `<img>` (see §13.4) |
| Folder upload | `<input type="file" webkitdirectory>` and drag-drop with `DataTransferItem.webkitGetAsEntry()` where supported; graceful degradation to file-only upload on browsers without directory APIs |

---

## 5. Repository layout

```text
/
  docs/
    PROJECT_SPEC.md
  server/
    cmd/cloud/main.go
    internal/
      config/         environment loading
      db/             pgx pool, transaction helpers
      httpapi/        router, middleware, error writer
      auth/           passwords, tokens, lockout, invites
      identity/       users, admin user management
      tree/           folders, names, move/rename
      files/          upload, download, range, collision, versions
      transfer/       zip streaming, throttle, quota
      favorites/
      trash/
      preview/        thumbnail jobs
      admin/          settings, logs, rebuild
      audit/          privacy-preserving system_logs writer
      ws/             hub and event types
      jobs/           nightly and interval workers
      storage/        disk paths, sidecars, atomic rename
    migrations/
    Dockerfile
  web/
    index.html
    src/                React UI, API client, hooks
    public/
    vite.config.ts
  deploy/
    docker-compose.yml
    .env.example
```

No application file is created until Phase 1 is approved. `docs/PROJECT_SPEC.md` is the only file this step produces.

---

## 6. Disk layout

Root: `/data` (compose volume `LOCAL_CLOUD_data`).

```text
/data/
  users/
    {user_id}/
      objects/
        {file_id}/
          v{version_num}          raw bytes, no extension required
          thumb.jpg                present only when thumbnail_status = ready
          meta.json                server-written sidecar
      tmp/
        {upload_session_id}.part  inbound stream; renamed into vN on commit
  backups/
    vault-YYYYMMDDTHHMMSSZ.dump  pg_dump custom format
  tmp/                            zip scratch only if a stream cannot be pure; prefer no scratch
```

Rules:

- Client-supplied names never become path segments. Paths are built only from server UUIDs and integer version numbers.
- A completed upload is `fsync` of the temp file, then atomic rename into `v{n}`, then the database transaction, then `meta.json` write. If the transaction fails, the new blob is removed. If the process dies between rename and commit, the nightly reconciler or Rebuild reports an unreferenced blob and does not delete it automatically.
- `meta.json` fields: `file_id`, `user_id`, `version_num`, `byte_size`, `sha256`, `mime_type`, `name`, `folder_path` (array of folder names from the user root down to the parent), `meta_created_at`, `meta_modified_at`, `uploaded_at`. Folder path is denormalized for disaster recovery only.
- Trash does not move blobs. Hard delete removes `objects/{file_id}/` entirely.
- Overwrite of a versioned file writes `v{n+1}` and leaves older `v*` until the retention job removes them.

---

## 7. Database schema (PostgreSQL 16)

The logical entities below are required. Column types and the extra columns are normative: they are the minimum needed to enforce the features in this document. Phase 1 migrations must create these objects. Do not invent a parallel schema in later phases.

Conventions:

- Primary keys are `UUID`.
- Timestamps are `TIMESTAMPTZ`, UTC.
- `BIGINT` byte counts. No floating storage units in the database.
- Foreign keys are `ON DELETE CASCADE` from a user to that user’s tree, except logs, which use `ON DELETE SET NULL` so a removed user’s audit lines remain until the 30-day purge.
- All queries are parameterized.

### 7.1 `users`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `username` | TEXT UNIQUE NOT NULL | 3–32 chars, `^[a-z0-9][a-z0-9._-]*$`, stored lowercase |
| `password_hash` | TEXT NOT NULL | Argon2id PHC string |
| `is_admin` | BOOLEAN NOT NULL DEFAULT false | At least one admin must remain |
| `is_disabled` | BOOLEAN NOT NULL DEFAULT false | Admin kill switch. Disabled users cannot authenticate |
| `storage_quota_bytes` | BIGINT NOT NULL | Must be `>= 0`. `0` means no uploads |
| `storage_used_bytes` | BIGINT NOT NULL DEFAULT 0 | Denormalized sum of live blobs + version blobs + trashed blobs + thumbnails. Updated in the same transaction as the blob change. A nightly job repairs drift |
| `speed_limit_kbps` | INTEGER NULL | Null inherits the global limit. `0` means unlimited for this user even if a global cap exists. Positive value is kilobytes/second (decimal KB, 1000 bytes) |
| `failed_login_attempts` | INTEGER NOT NULL DEFAULT 0 | |
| `locked_until` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ NOT NULL | |
| `updated_at` | TIMESTAMPTZ NOT NULL | |
| `last_login_at` | TIMESTAMPTZ NULL | |

### 7.2 `invite_codes`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `code_hash` | TEXT UNIQUE NOT NULL | SHA-256 of the code. The plaintext is shown once at creation and never stored |
| `code_hint` | TEXT NOT NULL | Last 4 characters, for the admin list |
| `created_by` | UUID NOT NULL FK → users | |
| `is_used` | BOOLEAN NOT NULL DEFAULT false | |
| `used_by` | UUID NULL FK → users | |
| `expires_at` | TIMESTAMPTZ NOT NULL | Default 7 days from creation |
| `default_quota_bytes` | BIGINT NULL | Null → `settings.default_user_quota_bytes` at redemption |
| `used_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ NOT NULL | |

There is no open registration. `POST /auth/register` requires a valid unused unexpired code. Redemption and user creation are one transaction: insert user, insert root folder, mark code used. A used or expired code fails with the same generic error as an unknown code.

### 7.3 `folders`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL FK → users | |
| `parent_id` | UUID NULL FK → folders | Null only for the single root folder |
| `name` | TEXT NOT NULL | Empty string only for the root. Otherwise 1–255 chars |
| `is_deleted` | BOOLEAN NOT NULL DEFAULT false | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `deleted_batch_id` | UUID NULL | Groups a cascade so restore does not revive separately deleted children |
| `created_at` | TIMESTAMPTZ NOT NULL | |
| `updated_at` | TIMESTAMPTZ NOT NULL | |

Constraints:

- Exactly one root per user: unique partial index on `user_id` WHERE `parent_id IS NULL`.
- Among non-deleted siblings, names are unique case-insensitively: unique index on `(user_id, parent_id, lower(name))` WHERE `is_deleted = false` AND `parent_id IS NOT NULL`.
- A folder cannot be its own ancestor. Moves are checked in application code with a recursive walk, and rejected with `CYCLE`.
- Name rules for non-root folders and for files: reject NUL, ASCII control characters, `/`, `\`, and the Windows-forbidden set `<>:"|?*`. Reject names that are `.` or `..`, that end in space or dot, and that match a Windows reserved device name (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`) ignoring extension. Original casing is stored.

Every user has a root folder created in the same transaction as the user. File and folder APIs address children by parent id. The UI does not show the root’s empty name; it shows “Cloud”.

### 7.4 `files`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL FK → users | |
| `folder_id` | UUID NOT NULL FK → folders | Always a real folder, including root |
| `name` | TEXT NOT NULL | Same character rules as folders |
| `size_bytes` | BIGINT NOT NULL | Size of the **current** version |
| `mime_type` | TEXT NOT NULL | Server-detected type (see §9.4) |
| `disk_path` | TEXT NOT NULL | Relative to `/data`, current version blob |
| `file_hash` | TEXT NOT NULL | Lowercase hex SHA-256 of current bytes |
| `is_deleted` | BOOLEAN NOT NULL DEFAULT false | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `deleted_batch_id` | UUID NULL | |
| `meta_created_at` | TIMESTAMPTZ NULL | Original OS creation time. Null if the client could not read it |
| `meta_modified_at` | TIMESTAMPTZ NULL | Original OS modification time |
| `thumbnail_path` | TEXT NULL | Relative to `/data` |
| `thumbnail_status` | TEXT NOT NULL DEFAULT `none` | `none`, `pending`, `ready`, `failed`, `skipped` |
| `created_at` | TIMESTAMPTZ NOT NULL | Server receive time |
| `updated_at` | TIMESTAMPTZ NOT NULL | |

Unique among non-deleted siblings, case-insensitive, same index pattern as folders.

`created_at` is ingest time. `meta_created_at` / `meta_modified_at` are sent by the browser during upload when available (`File.lastModified` → `meta_modified_at`; creation time is often unavailable in browsers and may be null). The UI labels them “Date modified” and “Date created” when present.

### 7.5 `file_versions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `file_id` | UUID NOT NULL FK → files | |
| `version_num` | INTEGER NOT NULL | Starts at 1. Unique per file |
| `size_bytes` | BIGINT NOT NULL | |
| `disk_path` | TEXT NOT NULL | |
| `file_hash` | TEXT NOT NULL | |
| `created_at` | TIMESTAMPTZ NOT NULL | When this version became current |

The current version is the row with the greatest `version_num`. `files.disk_path` matches that row.

Version policy:

- History is kept only when **both** the superseded blob and the new blob are strictly under 100 MiB (`104,857,600` bytes). The threshold lives in `settings.version_max_bytes` and defaults to that value.
- When history applies, the previous blob stays on disk and remains a `file_versions` row. The new blob is the next `version_num`.
- When history does not apply, older version blobs for that file are deleted as part of the successful overwrite transaction’s cleanup (delete files after commit). The new blob is the only remaining version.
- Non-current versions older than `settings.version_retention_days` (default 7) are removed by the nightly job. The current version is never removed by retention.
- Soft delete keeps version blobs. Hard delete removes them all.
- Restoring an old version (files that still have history): the restored bytes become a new current version. If the version policy applies, the previous current is retained; otherwise it is discarded.

### 7.6 `favorites`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL FK → users | |
| `file_id` | UUID NULL FK → files | |
| `folder_id` | UUID NULL FK → folders | |
| `created_at` | TIMESTAMPTZ NOT NULL | |

Check: exactly one of `file_id` or `folder_id` is non-null. Unique `(user_id, file_id)` where `file_id` is not null. Unique `(user_id, folder_id)` where `folder_id` is not null. The root folder cannot be favorited. A favorite pointing at a trashed object stays in the table and is hidden from the default favorites list; restore makes it visible again.

### 7.7 `system_logs`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NULL FK → users | Actor. Null for system jobs |
| `event_type` | TEXT NOT NULL | Closed vocabulary in §12 |
| `message` | TEXT NOT NULL | Privacy-preserving template. No filenames, paths, hashes, passwords, tokens, or invite codes |
| `ip_address` | INET NULL | Client IP as seen by the API. Null for internal jobs |
| `created_at` | TIMESTAMPTZ NOT NULL | |

Index on `created_at`. Rows older than `settings.log_retention_days` (default 30) are deleted nightly. Logs are visible to admins only.

Message examples:

- `uploaded 1048576 bytes into folder 018f…`
- `login failed`
- `account locked for 15 minutes`
- `rebuild finished: 10 files scanned, 0 orphans, 0 missing`

### 7.8 `refresh_tokens`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL FK → users | |
| `token_hash` | TEXT UNIQUE NOT NULL | SHA-256 |
| `device_label` | TEXT NULL | Client-supplied hostname, truncated to 64 chars |
| `expires_at` | TIMESTAMPTZ NOT NULL | |
| `revoked_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ NOT NULL | |

Logout revokes the presented refresh token. Password change and account disable revoke all refresh tokens for that user.

### 7.9 `settings`

Single row. `id BOOLEAN PRIMARY KEY DEFAULT true` with `CHECK (id)`.

| Column | Default | Meaning |
|---|---|---|
| `global_speed_limit_kbps` | NULL | Null or `0` = no global cap. Else decimal KB/s applied to every transfer |
| `default_user_quota_bytes` | 107374182400 (100 GiB) | Used when an invite does not override |
| `trash_retention_days` | 30 | |
| `version_retention_days` | 7 | |
| `version_max_bytes` | 104857600 | |
| `log_retention_days` | 30 | |
| `updated_at` | now() | |

v1 exposes admin edits for the global speed limit and per-user quota and speed. Retention numbers are constants in the product behavior (30 and 7). They live in this row so jobs have one place to read them. Changing retention days is not a v1 admin feature unless a later decision says so.

### 7.10 `upload_sessions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL FK → users | |
| `folder_id` | UUID NOT NULL FK → folders | |
| `file_id` | UUID NULL | Set when the session targets an overwrite |
| `name` | TEXT NOT NULL | Final name after collision policy |
| `expected_size_bytes` | BIGINT NULL | If provided, used for quota pre-check and progress |
| `received_bytes` | BIGINT NOT NULL DEFAULT 0 | |
| `temp_path` | TEXT NOT NULL | |
| `mime_hint` | TEXT NULL | Client hint only |
| `meta_created_at` | TIMESTAMPTZ NULL | |
| `meta_modified_at` | TIMESTAMPTZ NULL | |
| `status` | TEXT NOT NULL | `open`, `completed`, `aborted`, `expired` |
| `expires_at` | TIMESTAMPTZ NOT NULL | Default 24 hours from creation. Expired open sessions are aborted nightly and their temp files deleted |
| `created_at` | TIMESTAMPTZ NOT NULL | |

### 7.11 `thumbnail_jobs`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `file_id` | UUID NOT NULL FK → files | |
| `status` | TEXT NOT NULL | `pending`, `running`, `done`, `failed` |
| `attempts` | INTEGER NOT NULL DEFAULT 0 | |
| `available_at` | TIMESTAMPTZ NOT NULL | |
| `last_error` | TEXT NULL | Internal error text. Not copied verbatim into `system_logs` if it contains paths; store a short code there instead |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Eligible MIME groups: `image/jpeg`, `image/png`, `image/gif`, `image/webp`, `video/mp4`, `video/webm`, `video/quicktime`. Other types are `thumbnail_status = skipped`. Output is a JPEG, longest edge 480 px. Video frame is taken at 1 second, or at 0 if the file is shorter. Failure retries up to 3 times with backoff (1 min, 5 min, 15 min), then `failed`. Upload HTTP responses do not wait for ffmpeg.

### 7.12 Indexes and cascade summary

Required indexes beyond the unique ones above:

- `files (user_id, folder_id) WHERE is_deleted = false`
- `files (user_id, deleted_at) WHERE is_deleted = true`
- `folders (user_id, parent_id) WHERE is_deleted = false`
- `file_versions (file_id, version_num DESC)`
- `file_versions (created_at) WHERE version_num` is not filtered; the job deletes non-current rows by age
- `upload_sessions (status, expires_at)`
- `thumbnail_jobs (status, available_at)`
- `refresh_tokens (user_id)`

Deleting a user (admin, and only if not the last admin) removes their folders, files, favorites, tokens, invites they created remain with `created_by` handled as `ON DELETE RESTRICT` until those invites are deleted or the FK is set to another admin. Phase 2 implements: reassign `invite_codes.created_by` is not required; block user delete while unused invites they created exist, and set `used_by` behavior so historical used invites do not block — `created_by` ON DELETE RESTRICT, and the admin UI requires revoking unused invites first. Disk cleanup for that user runs after the database transaction commits.

---

## 8. Authentication and accounts

### 8.1 Bootstrap

If the `users` table is empty at process start, create one admin from:

- `LOCAL_CLOUD_ADMIN_USERNAME`
- `LOCAL_CLOUD_ADMIN_PASSWORD` (defaults to `admin` with username `admin`; change in production)
- `LOCAL_CLOUD_ADMIN_QUOTA_BYTES` (default 100 GiB)

If the table is empty and those variables are missing, the process refuses to become ready. There is no built-in default password.

### 8.2 Login lockout

- Track failures on the **account**, in the database, so restarts do not clear them.
- Unknown username: same 401 body as a bad password (`INVALID_CREDENTIALS`). Increment nothing (no account). Also apply a per-IP limiter of 20 login attempts per 15 minutes so the endpoint cannot be used as an oracle at line rate. Per-IP state may be in memory.
- Known user, wrong password: increment `failed_login_attempts`. On the 5th consecutive failure set `locked_until = now() + 15 minutes` and write `auth.lockout`.
- While `locked_until` is in the future: `423 LOCKED` with `retry_after_seconds`. Do not increment further. Do not check the password (prevents timing tricks from extending work, and matches “locked”).
- Successful login: set attempts to 0, clear `locked_until`, set `last_login_at`.
- Disabled user: `403 ACCOUNT_DISABLED` after the password check succeeds. Wrong password on a disabled account still counts as a failure.

### 8.3 Tokens

- `POST /auth/login` and `POST /auth/register` return `access_token`, `access_expires_at`, `refresh_token`, and the user profile.
- `POST /auth/refresh` accepts the refresh token, revokes it, inserts a new one, and returns a new pair. Reuse of a revoked refresh token revokes **all** refresh tokens for that user (theft signal) and returns 401.
- Access token expiry forces a refresh. The SPA HTTP wrapper refreshes proactively (for example 60 seconds before `exp`) and once on `401`, then retries the original request.

### 8.4 Invite flow

1. Admin creates a code. API returns the plaintext once.
2. New user submits username, password, invite code.
3. Server hashes the code, locks the invite row, checks unused and unexpired, creates the user and root folder, consumes the invite.
4. Password policy: 12–128 characters. No composition rules beyond length.

---

## 9. File, folder, and transfer behavior

### 9.1 What “sync state” means

Sync state is the state of an **in-browser transfer job**, not a binding between a local directory and a remote directory.

Transfer states in the UI: `queued`, `running`, `completed`, `failed`, `canceled`.

There is no filesystem watcher and no background sync agent in v1.

### 9.2 Uploads

Drag-and-drop of files or folders is a browser feature.

- Folder upload: walk dropped directory entries in JavaScript (or use `webkitdirectory` on the file input). The SPA creates missing remote folders (parent before child), then uploads files. Concurrent uploads per browser tab: 3.
- Timestamps: send `meta_modified_at` from `File.lastModified` when present. Send `meta_created_at` only when the browser exposes it (often null).
- Protocol (bounded memory on the server):
  1. `POST /api/v1/uploads` with JSON metadata. Server pre-checks quota when `expected_size_bytes` is present: `storage_used_bytes - size_being_replaced + expected_size <= quota`. Collision is evaluated here.
  2. `PUT /api/v1/uploads/{id}/content` with raw body. Optional header `Offset` (integer). Server seeks/truncates to that offset and streams `io.Copy` from the request body to the temp file. It does not buffer the body. `Offset` must equal `received_bytes` (no sparse holes).
  3. `POST /api/v1/uploads/{id}/complete` streams the temp file through SHA-256 (or hashes during the PUT—hash during PUT is required so complete does not re-read), fsyncs, renames, commits metadata, writes `meta.json`, enqueues a thumbnail job if eligible, emits a WebSocket event.
- If the connection drops, the client retries the PUT at `received_bytes`. Sessions die at `expires_at`.
- Directory upload failure is per file. Successfully created folders remain. The UI shows which files failed.

### 9.3 Collisions

A collision is another **non-deleted** file in the same folder whose name compares equal under Unicode case-folding (use a simple ASCII case-fold plus exact match for non-ASCII; document implementation as `lower()` in Postgres for v1, which is database-locale dependent—**the database must be created with `LOCALE=C` or `ICU` deterministic lowercase, and the compose Postgres sets `lc_collate=C`**). v1 comparison is byte-wise lowercase of ASCII, with non-ASCII compared case-sensitively if Postgres `lower()` would be locale-unsafe. Normative rule: **case-insensitive for ASCII, case-sensitive for other code points**, implemented in Go and mirrored by a stored `name_key` column.

Add column `name_key TEXT NOT NULL` on `files` and `folders`, computed by the server with that rule. Unique indexes use `name_key`, not `lower(name)`.

On collision, `POST /uploads` returns `409 CONFLICT` and does not create a session. Body includes the existing file’s id, size, and `updated_at`. The client prompts:

- **Overwrite** — retry init with `conflict_policy=overwrite` and `file_id`.
- **Rename** — user picks a new name; retry init.
- **Cancel** — no request.

The prompt is mandatory. The client must not overwrite silently.

### 9.4 MIME detection

The server sniffs the first 512 bytes with the Go detect function and stores that result. If the result is `application/octet-stream`, the server may map a small extension allowlist solely to decide preview eligibility (`mp4`, `webm`, `mov`, `mkv` → treat as video for thumbnail and range; the stored `mime_type` becomes `video/mp4` only for `mp4`, otherwise `video/webm` for `webm`, and `video/quicktime` for `mov`; `mkv` is `video/x-matroska` and is range-downloadable but thumbnail-eligible). The client hint is never stored as the authority.

### 9.5 Downloads

- Single file: `GET /api/v1/files/{id}/content`.
  - Supports `Range: bytes=start-end` (single range only). `206` with `Content-Range`, or `200` for a full GET. `Accept-Ranges: bytes`. Invalid range → `416`.
  - Auth: `Authorization: Bearer` **or** query parameter `mt=<media_token>` (see §13.4). Media tokens exist so `<video>` and `<img>` can seek without custom headers.
  - Video preview uses this endpoint with a media token. Playback must start without downloading the whole object.
  - `Content-Disposition` uses the stored name via `filename*` (RFC 5987). The on-disk path is never leaked.
- Folder, or mixed selection: `POST /api/v1/downloads/archive` with `{ "folder_ids": [], "file_ids": [] }`. Response is `application/zip`, streamed. Entry names come from the database tree. Path segments are the sanitized display names. Zip-slip is prevented by construction (no `..`, no absolute paths, no drive letters). A selection that would exceed a configured guard of 100,000 entries is rejected with `413`. Compression method is store when the MIME is already compressed (zip, jpeg, png, mp4, webm, gzip, and similar); otherwise deflate.
- Trash and other users’ ids return `404`, not `403`, so existence is not leaked across users. Within the owner, authorization failures of this kind are `404`.

### 9.6 Quotas

`storage_used_bytes` includes current blobs, historical version blobs, trashed blobs, and thumbnail bytes. It does not include open upload temp files until complete; the pre-check uses `expected_size_bytes` so a user cannot start a transfer that cannot finish. If `expected_size_bytes` is omitted, the server still enforces quota at completion and deletes the temp file on `507 QUOTA_EXCEEDED`.

A user at quota can still delete and can still download.

### 9.7 Speed limits

Effective limit for a transfer:

1. If the user’s `speed_limit_kbps` is `0`, the transfer is unlimited.
2. Else if the user’s limit is a positive number and the global limit is positive, effective = minimum of the two.
3. Else use whichever of user or global is positive.
4. Else unlimited.

Enforcement is a token bucket around the `io.Copy` reader or writer for that request, in the API process. Limits are per request, not a shared pipe across a user’s parallel requests in v1 (documented limitation). Global limit is also per request. Kilobytes are decimal (1000 bytes/second). `speed_limit_kbps = 500` means 500,000 bytes/second.

### 9.8 Browser limitations (v1)

- **No OS lock retry:** the web app reads files the user selects or drops. If a file is locked by another program, the read fails immediately; the transfer shows `failed` and the user can retry manually after closing the other program. There is no 1-hour or nightly retry schedule (that behavior belonged to the retired desktop design).
- **No vault cache quota:** the server holds all bytes. The browser may cache thumbnails and media in memory for the open session only. Large files are downloaded via the browser’s normal save flow, not into a managed vault cache on disk.
- **Tab close:** closing the tab cancels in-flight uploads unless the browser continues them (not relied upon). The upload session on the server remains resumable until `expires_at` if the user re-uploads with the same session id stored in `sessionStorage` (optional enhancement: persist active session ids per file in `sessionStorage`).

### 9.9 Trash

- Delete of a file sets `is_deleted`, `deleted_at`, `deleted_batch_id`.
- Delete of a folder does the same for the folder and for every not-yet-deleted descendant, sharing one new `deleted_batch_id`. Descendants that were already in trash keep their original batch id and timestamp.
- Trash listing shows top-level trashed items: a file or folder whose parent is not in the same deleted batch (so a trashed folder appears once, not as every child).
- Restore of a file requires a non-deleted parent. If the parent is deleted, the API returns `409 PARENT_DELETED`. The client may offer “Restore to cloud root”, which is a second call with `destination_folder_id` of the root.
- Restore checks name collision in the destination and uses the same Overwrite / Rename / Cancel prompt.
- Restore of a folder restores rows with the same `deleted_batch_id` under it.
- Nightly job hard-deletes rows with `deleted_at` older than 30 days. Folder hard-delete is depth-safe: delete children blobs first, then rows. Hard delete is permanent.
- The owner may hard-delete one trash item early (`DELETE /api/v1/trash/{kind}/{id}`). Admins do not browse other users’ file names through this API.

### 9.10 Real-time events

`GET /api/v1/ws` upgrades to WebSocket. The first client message, within 5 seconds, is:

```json
{ "type": "auth", "access_token": "<jwt>" }
```

Failure closes the socket. After auth, the server sends:

```json
{ "type": "ready" }
```

Server pushes, to every live connection for that **same user id**:

| type | When |
|---|---|
| `file.created` | Upload complete or restore that yields a new visible file |
| `file.updated` | Rename, move, overwrite, version restore |
| `file.deleted` | Soft delete |
| `file.thumbnail_ready` | Thumbnail job succeeded |
| `folder.created` | |
| `folder.updated` | |
| `folder.deleted` | |
| `trash.changed` | Restore or hard delete |
| `account.updated` | Quota or speed limit changed |
| `favorite.changed` | |

Payloads include the public resource DTO, not disk paths. The client that caused the change also receives the event and must reconcile without double-inserting (upsert on id).

Ping every 30 seconds. Missed pong: server closes. The client reconnects with exponential backoff capped at 30 seconds and then refetches the open folder.

---

## 10. HTTP API

Base path: `/api/v1`. JSON errors:

```json
{
  "error": {
    "code": "QUOTA_EXCEEDED",
    "message": "Storage quota would be exceeded.",
    "details": {}
  }
}
```

`message` is safe to show in the UI. `details` is optional structured data (ids, retry seconds), never stack traces.

| Status | Codes |
|---|---|
| 400 | `VALIDATION`, `BAD_RANGE` |
| 401 | `UNAUTHENTICATED`, `INVALID_CREDENTIALS` |
| 403 | `FORBIDDEN`, `ACCOUNT_DISABLED` |
| 404 | `NOT_FOUND` |
| 409 | `CONFLICT`, `PARENT_DELETED`, `CYCLE` |
| 413 | `TOO_LARGE` |
| 416 | `RANGE_NOT_SATISFIABLE` |
| 423 | `LOCKED` |
| 507 | `QUOTA_EXCEEDED` |
| 500 | `INTERNAL` |

List endpoints use cursor pagination: `?limit=100&cursor=`. Default limit 100, max 500. Response: `{ "items": [], "next_cursor": null }`.

### 10.1 Public and session

| Method | Path | Auth | Behavior |
|---|---|---|---|
| GET | `/healthz` | no | Process is up |
| GET | `/readyz` | no | Database ping succeeds |
| POST | `/auth/register` | no | Invite redemption. Body: `username`, `password`, `invite_code`, `device_label` |
| POST | `/auth/login` | no | Body: `username`, `password`, `device_label` |
| POST | `/auth/refresh` | refresh token in body | Rotates refresh token |
| POST | `/auth/logout` | bearer | Body contains refresh token to revoke |
| GET | `/auth/me` | bearer | Profile, quota, used bytes, effective speed limit, root folder id |

### 10.2 Tree and files (owner)

| Method | Path | Behavior |
|---|---|---|
| GET | `/folders/{id}` | Folder DTO plus breadcrumb |
| GET | `/folders/{id}/children` | Subfolders and files in that folder. Query `include` not required; one combined payload `{ "folders": [], "files": [] }` is easier for the UI and is the chosen shape. Paginate files if needed; folders in a single folder are returned in full up to 5,000, then `400` |
| POST | `/folders` | Body: `parent_id`, `name` |
| PATCH | `/folders/{id}` | Rename and/or move. Body: optional `name`, optional `parent_id`. Root cannot be patched |
| DELETE | `/folders/{id}` | Soft delete cascade |
| POST | `/uploads` | Start session. Body: `folder_id`, `name`, `expected_size_bytes`, `mime_hint`, `meta_created_at`, `meta_modified_at`, optional `conflict_policy`, optional `file_id` |
| PUT | `/uploads/{id}/content` | Raw stream. Header `Offset` |
| POST | `/uploads/{id}/complete` | Finalize |
| DELETE | `/uploads/{id}` | Abort and delete temp |
| GET | `/files/{id}` | Metadata DTO |
| GET | `/files/{id}/content` | Download or range stream. Auth: bearer or `?mt=` media token |
| POST | `/files/{id}/media-token` | Returns `{ "token", "expires_at" }` — short-lived JWT scoped to read this file (and its thumbnail). Used by `<video>` / `<img>` |
| GET | `/files/{id}/thumbnail` | JPEG or `404` if not ready. Auth: bearer or `?mt=` |
| PATCH | `/files/{id}` | Rename and/or move |
| POST | `/files/{id}/copy` | Duplicate file bytes into optional `folder_id` (default: same folder). Name gets ` - Copy` suffix when needed |
| POST | `/folders/{id}/copy` | Deep-copy folder tree into optional `parent_id` (default: same parent). Root folder cannot be copied |
| DELETE | `/files/{id}` | Soft delete |
| GET | `/files/{id}/versions` | Version list for the owner |
| POST | `/files/{id}/versions/{version_id}/restore` | Make that version current under the policy in §7.5 |
| POST | `/downloads/archive` | Stream a zip |
| GET | `/trash` | Trash tops |
| POST | `/trash/files/{id}/restore` | Optional body `destination_folder_id`, `conflict_policy`, `name` |
| POST | `/trash/folders/{id}/restore` | Same |
| DELETE | `/trash/files/{id}` | Early hard delete |
| DELETE | `/trash/folders/{id}` | Early hard delete of that batch under the folder |
| GET | `/favorites` | Visible favorites |
| POST | `/favorites` | Body: exactly one of `file_id`, `folder_id` |
| DELETE | `/favorites/{id}` | |
| GET | `/search` | Query `q`. Matches `name` within the caller’s non-deleted files and folders. Case-insensitive ASCII contains. Limit 50. v1 search is `ILIKE` on name, escaped for `%` and `_`. No full text index required |

File DTO (API and WebSocket): `id`, `folder_id`, `name`, `size_bytes`, `mime_type`, `file_hash`, `meta_created_at`, `meta_modified_at`, `created_at`, `updated_at`, `thumbnail_status`, `is_favorite`. Never `disk_path`.

### 10.3 Admin

All routes require `is_admin`. An admin acting on files still uses the owner routes and only sees their own tree. Admin routes do not accept a `user_id` to download another person’s files. That absence is intentional.

| Method | Path | Behavior |
|---|---|---|
| GET | `/admin/users` | List users with quota, used bytes, speed limit, disabled flag, last login. No password hashes |
| PATCH | `/admin/users/{id}` | `storage_quota_bytes`, `speed_limit_kbps`, `is_admin`, `is_disabled`. Cannot disable or demote the last admin. Cannot set quota below `storage_used_bytes` (reject `400`; admin must not silently delete data) |
| DELETE | `/admin/users/{id}` | Deletes the account and, after commit, deletes `/data/users/{id}`. Blocked for self and for the last admin |
| POST | `/admin/invites` | Body: optional `expires_at`, optional `default_quota_bytes`. Returns plaintext `code` once |
| GET | `/admin/invites` | Lists hint, expiry, used flag, used_by username. Never the full code |
| DELETE | `/admin/invites/{id}` | Revoke unused. Used codes stay for audit and return `409` |
| GET | `/admin/settings` | Global speed limit and retention constants (read-only constants) |
| PATCH | `/admin/settings` | Body: `global_speed_limit_kbps` |
| GET | `/admin/logs` | Cursor pagination, newest first. Query: `event_type`, `user_id`, `from`, `to` |
| GET | `/admin/stats` | User count, total used bytes, total quota, open upload sessions |
| POST | `/admin/rebuild` | Starts database rebuild. Returns `202` and a job id. One rebuild at a time; a second call returns `409` |

### 10.4 WebSocket

`GET /api/v1/ws` as specified in §9.10.

---

## 11. Rebuild database (disaster recovery)

Two different mechanisms:

1. **Nightly logical backup.** `pg_dump -Fc` into `/data/backups/`, retain 7 dumps, delete older. This is the preferred recovery path and is documented in the admin UI as “restore a SQL backup with the operator runbook”, not as a one-click overwrite (one-click restore of Postgres is too dangerous for v1).
2. **Rebuild from disk sidecars.** Admin action for the case where Postgres metadata was lost but `/data/users` remains. It is the feature named “Rebuild Database”.

Rebuild algorithm (server job):

1. Refuse to run unless the caller is an admin. Write `admin.rebuild_started`.
2. Set a process-wide lock.
3. For each `/data/users/{user_id}/objects/{file_id}/meta.json` that parses:
   - Require `user_id` directory name to match the sidecar.
   - Find or create the user row **only if** the user row already exists. Rebuild does not invent accounts. Orphan directories whose user id is unknown are counted and skipped.
   - Recreate the folder chain from `folder_path` under that user’s root (case-insensitive ASCII match on each segment).
   - Insert a file row if that `file_id` is missing. If the row exists, compare size and hash; on mismatch, prefer the blob on disk, update the row, and count a repair. Do not delete the blob.
   - Attach version rows for each `v{n}` file present. Hash by streaming. Missing current blob: count as missing, leave the row if it exists, do not invent bytes.
4. Database rows whose `disk_path` is absent: count as missing. Do not delete those rows in v1. The job report lists counts only, plus up to 100 file ids, not names.
5. Unreferenced blobs with no valid sidecar: count as orphans. Do not delete them.
6. Recompute `storage_used_bytes` for every user from version blob sizes plus thumbnail sizes.
7. Write `admin.rebuild_finished` with counts. Store the last report in memory and return it from `GET /admin/rebuild` (last result). Persist the summary message in `system_logs`.

Rebuild never drops the database by itself. If the operator needs an empty metadata database first, that is an operator step outside the button: restore schema via migrations, then press Rebuild.

---

## 12. Background jobs

All jobs run inside the `localcloud-api` process. A single replica is the supported deployment. Jobs take a Postgres advisory lock so a future second replica would not double-run.

| Job | Schedule | Action |
|---|---|---|
| Trash purge | Nightly 02:15 UTC | Hard-delete trash older than 30 days |
| Version purge | Nightly 02:30 UTC | Remove non-current versions older than 7 days when history rules say they exist |
| Log purge | Nightly 02:45 UTC | Delete `system_logs` older than 30 days |
| Upload expiry | Every 15 min | Abort expired sessions, delete temp files |
| Thumbnail worker | Continuous poll, 1 s idle sleep | Claim one pending job, run ffmpeg, mark ready, emit WebSocket |
| Usage reconcile | Nightly 03:00 UTC | Recompute `storage_used_bytes` |
| Backup | Nightly 03:15 UTC | `pg_dump` and rotate to 7 files |

`event_type` vocabulary:

`auth.login.success`, `auth.login.failure`, `auth.lockout`, `auth.logout`, `auth.register`, `invite.created`, `invite.revoked`, `file.uploaded`, `file.downloaded`, `file.deleted`, `file.restored`, `file.renamed`, `file.moved`, `file.version_restored`, `folder.created`, `folder.deleted`, `folder.renamed`, `folder.moved`, `trash.purged`, `version.pruned`, `admin.user_updated`, `admin.user_deleted`, `admin.settings_updated`, `admin.rebuild_started`, `admin.rebuild_finished`.

Download audit: log once per completed full-file download and once per archive request, not once per HTTP range slice (video scrubbing must not flood the log).

---

## 13. Web client specification

### 13.1 Screens (Phase 4 and Phase 5)

- **Login** and **Create account** (invite code, not open registration). Unauthenticated users hitting `/` redirect to `/login`.
- **Vault:** breadcrumb, folder grid/list, favorites filter, search, drag-and-drop target, multi-select, download, delete, rename, move, favorite.
- **Preview drawer:** thumbnail or full image via media token URL; `<video controls>` with `src` pointing at `/api/v1/files/{id}/content?mt=...` so Range seeking works without a full download.
- **Transfers:** panel with per-file progress, retry, cancel.
- **Trash.**
- **Settings:** logout only in v1 (no cache limit).
- **Admin** (only if `is_admin`): users, invites, global speed, logs, rebuild status and trigger.

Empty, error, and loading states are part of each screen, not follow-up work.

### 13.2 Frontend modules

- `api/client.ts` — fetch wrapper with bearer injection, refresh, and typed errors from §10.
- `api/auth.ts`, `api/files.ts`, `api/admin.ts` — route functions matching the REST table.
- `state/auth.tsx` — access token in React context; hydrate refresh from `sessionStorage` on load.
- `state/transfers.ts` — upload/download queue; optional persist active jobs to `sessionStorage`.
- `ws/cloud.ts` — WebSocket connect, auth message, event dispatch into TanStack Query cache.
- `components/` — cloud browser, modals (collision, move), preview, admin layout.

### 13.3 SPA routing and server fallback

Client routes are handled in the browser. The Go server serves `index.html` for any non-API `GET` that is not a static asset (SPA fallback) so deep links like `/cloud/folder/{id}` work on refresh.

Static assets use content-hashed filenames and long cache lifetimes. `index.html` is `Cache-Control: no-cache`.

### 13.4 Media tokens

HTML media elements cannot send `Authorization` headers. For preview and inline thumbnail display:

1. The SPA calls `POST /api/v1/files/{id}/media-token` with the normal bearer token.
2. The server returns a JWT (or opaque token) valid for **15 minutes**, claims limited to `file_id` and `purpose=media_read`, signed with the same secret as access tokens or a dedicated `LOCAL_CLOUD_MEDIA_SECRET`.
3. The SPA sets `src` to `/api/v1/files/{id}/content?mt=<token>` (and the same for `/thumbnail` when needed).
4. The content handler accepts either bearer or valid `mt`. Invalid or expired `mt` → `401`. The token must not appear in audit logs.

Range requests on media URLs must work with `mt` present (video seeking).

### 13.5 Docker image build

The production `localcloud-api` image build runs `npm ci && npm run build` in `web/`, then embeds or copies `web/dist` into the Go binary/image. A single container serves UI and API on the port from `LOCAL_CLOUD_HTTP_ADDR` (default 8090).

---

## 14. Deployment

### 14.1 Compose services

| Service | Image | Notes |
|---|---|---|
| `postgres` | `postgres:16` | Volume `pg_data` → `/var/lib/postgresql/data`. Not published to the host. `POSTGRES_INITDB_ARGS` with locale `C`. Healthcheck `pg_isready` |
| `localcloud-api` | built from `server/Dockerfile` | Depends on healthy postgres. Publishes **`${LOCAL_CLOUD_HOST_PORT:-8090}:${LOCAL_CLOUD_PORT:-8090}`** (see §14.2). Volume `LOCAL_CLOUD_data` → `/data`. Runs migrations on startup, then serves. Label `com.centurylinklabs.watchtower.enable=true` |
| `watchtower` | pinned `containrrr/watchtower` | Docker socket. Scope label above. Poll interval 24 h. Cleanup old images. No automatic Postgres updates |

### 14.2 Environment

Documented in `deploy/.env.example` (Phase 1). Never commit a real `.env`.

| Variable | Required | Purpose |
|---|---|---|
| `POSTGRES_USER` | yes | |
| `POSTGRES_PASSWORD` | yes | |
| `POSTGRES_DB` | yes | `vault` |
| `DATABASE_URL` | yes | `pgx` URL to the compose service hostname |
| `LOCAL_CLOUD_PORT` | no | TCP port the Go process listens on **inside** the container. Default **`8090`**. Must match the container side of the Compose `ports:` mapping unless you intentionally split host/container ports |
| `LOCAL_CLOUD_HOST_PORT` | no | Host LAN port Docker publishes. Default **`8090`** (same as `LOCAL_CLOUD_PORT`). Set this (or both) when 8090 is taken — e.g. `LOCAL_CLOUD_HOST_PORT=9000` with `LOCAL_CLOUD_PORT=8090` maps host **9000** → container **8090** |
| `LOCAL_CLOUD_HTTP_ADDR` | no | Listen address passed to Go, e.g. `:8090`. Default **derived from `LOCAL_CLOUD_PORT`** as `:${LOCAL_CLOUD_PORT}`. Override only if you need a nonstandard bind (rare) |
| `LOCAL_CLOUD_DATA_DIR` | yes | `/data` |
| `LOCAL_CLOUD_JWT_SECRET` | yes | At least 32 random bytes, hex or base64 |
| `LOCAL_CLOUD_ADMIN_USERNAME` | first boot | |
| `LOCAL_CLOUD_ADMIN_PASSWORD` | first boot | |
| `LOCAL_CLOUD_ADMIN_QUOTA_BYTES` | no | |
| `LOCAL_CLOUD_LOG_LEVEL` | no | default `info` |

**Port configuration (operator-facing):**

- Default LAN URL: **`http://<host-ip>:8090/`**. Do **not** default to 8080; many homelab hosts already bind 8080 (qBittorrent Web UI, proxies, etc.).
- Change the port by editing `deploy/.env` before `docker compose up`:
  - **Same port inside and out (usual case):** set `LOCAL_CLOUD_PORT=9000` and `LOCAL_CLOUD_HOST_PORT=9000` (or set only `LOCAL_CLOUD_PORT` and let Compose default `LOCAL_CLOUD_HOST_PORT` to the same value).
  - **Host port differs from container port:** set `LOCAL_CLOUD_HOST_PORT=9000` and `LOCAL_CLOUD_PORT=8090`; browse **`http://<host-ip>:9000/`**.
- `deploy/.env.example` must document these variables with comments. `deploy/docker-compose.yml` must use `${LOCAL_CLOUD_HOST_PORT:-8090}:${LOCAL_CLOUD_PORT:-8090}` and pass `LOCAL_CLOUD_HTTP_ADDR=:${LOCAL_CLOUD_PORT:-8090}` (or equivalent) into the `localcloud-api` service.
- Phase 1 Go `config` loader reads `LOCAL_CLOUD_PORT` and sets listen address when `LOCAL_CLOUD_HTTP_ADDR` is unset.
- The SPA uses **relative** `/api/v1` URLs, so changing the host port never requires rebuilding the frontend.

### 14.3 Process model

- One API container.
- Migrations run before listen.
- SIGTERM finishes in-flight requests up to 30 seconds, stops taking new uploads, and exits. Temp files from killed uploads are cleaned by the expiry job using `received` vs session rows, not by hoping the process stays up.

### 14.4 Image contents

The API image includes the static `ffmpeg` binary (or the distro package) and `ca-certificates` only if needed later. It does not include a shell user with a password. Run as a non-root user that can write `/data`.

---

## 15. Security requirements

- Passwords: Argon2id only. Never log them. Never return them.
- Invite codes: 128 bits from `crypto/rand`, encoded as base32 without padding, stored as SHA-256.
- Authorization: every tree query includes `user_id = caller`. Admin is not a backdoor into file bytes.
- Path traversal: reject any disk path that is not rooted under `/data/users/{that user}` after `filepath.Clean`.
- Zip entries: built from cleaned relative names only.
- JSON body cap: 1 MiB. The raw upload PUT is exempt and bounded by quota.
- Filename length cap: 255. Folder depth cap: 40. Reject deeper creates.
- SQL injection: parameterized queries only.
- Dependency pinning: Go module sums and a lockfile for the client.
- Secrets: `.env` is gitignored. Logs must not print `DATABASE_URL` or `LOCAL_CLOUD_JWT_SECRET`.
- User deletion and rebuild are audited.
- Rate limit login as specified. Other routes rely on LAN trust plus authentication in v1.

---

## 16. Observability

- Stdout JSON logs: request id, method, path template (not raw file names in query), status, duration, bytes in/out. No Authorization header.
- `/healthz` and `/readyz` for Compose.
- Admin stats endpoint for the panel.
- Audit trail is `system_logs`, retained 30 days, content rules in §7.7.

---

## 17. Testing bar per phase

Tests are part of the phase, not a later cleanup.

- Go unit tests for: name validation, `name_key`, lockout counter, quota arithmetic, range parser, zip path sanitizer, speed-limit selection, version retention decision (keep vs discard).
- Go integration tests against Postgres for: register consumes invite once, unique sibling names, soft delete hides rows, overwrite version policy under and over 100 MiB.
- Streaming test: upload a body larger than a small buffer (several MiB is enough in CI) and assert memory-oriented implementation by using `io.Copy` to disk; assert the stored hash.
- Web unit tests (Vitest): auth refresh wrapper, transfer state reducer, media URL builder.
- UI verification: exercise the changed flow in a browser against a running stack when a phase touches the interface.

Placeholder tests (`assert true`) do not count.

---

## 18. Engineering rules

1. No placeholder implementations. No `TODO` and no `Implement later` in committed code. A phase ships the behavior it checks off.
2. Git commits are professional and specific (`feat: add invite-only registration`). No AI attribution, no “generated by”, no `Co-authored-by` trailer for an assistant.
3. Do not commit `.env`, dumps, or file-byte fixtures of real user data.
4. One phase at a time. Do not start the next phase until the previous phase’s checklist is done and you have been asked to continue.
5. If implementation discovers this spec cannot be followed, stop and record the conflict. Do not silently redesign.
6. Conventional scope: server changes live under `server/` and `deploy/`; web UI under `web/`.

---

## 19. Decision log

| ID | Decision | Reason |
|---|---|---|
| D1 | Not a 1:1 sync engine | Product definition. Browser shows an explicit transfer queue; no managed offline vault cache |
| D2 | HTTP without TLS in v1 | Stated network constraint. LAN isolation is the boundary |
| D3 | Streaming upload via session + raw PUT, not buffered multipart | Multi-gigabyte uploads must not sit in RAM |
| D4 | `name_key` column for collisions | ASCII case-insensitive uniqueness without depending on Postgres locales beyond `C` |
| D5 | Root folder row per user | Avoids nullable `folder_id` and sentinel UUIDs |
| D6 | Version history only when both old and new blobs are under 100 MiB, kept 7 days | Stated retention rule, made deterministic at the boundary |
| D7 | Quota includes trash and old versions | Otherwise delete-and-refill defeats quotas |
| D8 | Admin cannot read other users’ file bytes or names through admin APIs | Privacy-preserving operations. Host root access still can read disk; that is the trusted-operator model |
| D9 | Audit messages omit filenames and hashes | Privacy-preserving 30-day logs |
| D10 | Invite plaintext shown once, stored hashed | A database leak must not yield live invite codes |
| D11 | Watchtower updates `localcloud-api` only | Automatic Postgres upgrades are an outage and migration risk |
| D12 | Sidecar `meta.json` plus nightly `pg_dump` | Rebuild has something to read if metadata is lost; dumps cover the common case better |
| D13 | WebSocket auth is the first message, not a query string | Tokens must not land in access logs |
| D14 | Single range requests only | Enough for media element seeking; multipart ranges add complexity without a v1 client |
| D15 | Per-request throttling, not a global token bucket shared across connections | Simpler and predictable. Documented so nobody “fixes” it into a shared shaper without a decision |
| D16 | Retired: OS lock retry schedule | Applied only to the removed Tauri client; web uploads fail fast with manual retry |
| D17 | Postgres is not published to the host | Reduces accidental LAN exposure of the database |
| D18 | Web-only client on LAN IP | Primary UX is `http://<host>:<port>/`; no desktop installer |
| D21 | Default port 8090, configurable | Avoids 8080 (common qBittorrent Web UI port); `LOCAL_CLOUD_PORT` / `LOCAL_CLOUD_HOST_PORT` in `.env` |
| D19 | Same-origin embedded SPA | Go serves UI + API on one port; relative `/api/v1` paths |
| D20 | Media tokens for preview | Lets `<video>` use Range without exposing the refresh token or fragile blob URLs |
| D22 | LAN-only client IP filter always on | Rejects non-private source IPs at the HTTP layer; complements firewall / no port-forward; not configurable |
| D23 | No remote/VPN access path | Product intent is physical LAN presence only; operators must not rely on VPN or port-forward; remote file access is out of scope |

---

## 20. Build phases

Check a box only when that item is implemented, tested, and matches this spec. Do not check a box for a stub.

### Phase 1 — Docker and database foundation

- [x] Create the repository layout from §5 (`server/`; `web/` can wait until Phase 4)
- [x] Go module, process entrypoint, config loader, slog
- [x] `GET /healthz` and `GET /readyz`
- [x] PostgreSQL 16 service with locale `C`, healthcheck, private network
- [x] SQL migrations for every table, index, and check in §7, including `name_key`, `settings` seed row, and constraints
- [x] Migration runner on API startup
- [x] `/data` volume mount and directory permissions for a non-root API user
- [x] `deploy/docker-compose.yml`, `deploy/.env.example`, root `.gitignore` — configurable **`LOCAL_CLOUD_PORT`** / **`LOCAL_CLOUD_HOST_PORT`** (default **8090**, not 8080)
- [x] Watchtower service scoped to the API container only
- [x] Dockerfile builds the API and includes ffmpeg
- [x] Bootstrap admin when `users` is empty; refuse ready state when the table is empty and bootstrap env is absent
- [x] `storage` package: safe path join under `/data`, directory creation for a user
- [x] Integration test: migrations apply cleanly against Compose Postgres; ready check fails when the database is down; bootstrap creates one admin and one root folder
- [x] Manual check: `docker compose up` from a clean volume, `/healthz` and `/readyz` succeed from another LAN host or the Docker host (UI route may 404 until Phase 4 embeds the SPA; that is expected)

### Phase 2 — Go auth and users

- [x] Argon2id hash and verify
- [x] Login, lockout (5 failures, 15 minutes), disabled-account behavior, generic unknown-user errors
- [x] Per-IP login rate limit
- [x] Invite create/list/revoke and one-time register transaction (user + root folder + consume code)
- [x] Access JWT and rotating refresh tokens, including reuse detection
- [x] Logout and `GET /auth/me`
- [x] Admin user list, patch quota / speed / admin / disabled, delete user with last-admin protection
- [x] Audit events for auth and admin user changes, privacy rules applied
- [x] Tests listed in §17 that belong to auth and invites
- [x] No file-tree routes in this phase beyond creating the root folder internally

### Phase 3 — Go file streaming and sockets

- [x] Folder create, list children, rename, move, cycle prevention, soft delete
- [x] Name validation and collision detection via `name_key`
- [x] Upload session, offset PUT streamed to disk, hash, complete, abort, expiry job
- [x] Original OS timestamps stored on the file row and in `meta.json`
- [x] Overwrite / rename collision policies and the version rule (under 100 MiB vs at-or-over)
- [x] Quota enforcement on init and on complete
- [x] Per-request speed limit selection and token bucket
- [x] Single-file download with HTTP Range
- [x] Thumbnail job queue and ffmpeg worker; `GET` thumbnail
- [x] Streaming zip of folders and mixed selections
- [x] Favorites
- [x] Trash list, restore, parent-deleted conflict, early hard delete (job schedule itself is Phase 6, but the delete **functions** exist now and are callable)
- [x] Search by name
- [x] WebSocket hub, first-message auth, event types in §9.10
- [x] Media token issuance and validation on file content + thumbnail routes
- [x] Download audit that ignores range slices
- [x] Tests: streaming hash, quota, range, zip names, version threshold, sibling uniqueness

### Phase 4 — Web app foundation

- [x] Vite + React + TypeScript + Tailwind under `web/`
- [x] Dev proxy to Go API; production build embedded or copied into `localcloud-api` image (§13.5)
- [x] Go static file handler + SPA fallback for non-API routes
- [x] Auth context: login, register, logout, refresh, `sessionStorage` for refresh token
- [x] Typed API client for Phase 2 and Phase 3 endpoints
- [x] WebSocket client, reconnect, query invalidation on cloud events
- [x] Media token fetch helper and preview URL builder (§13.4)
- [x] Upload pipeline: session init, streaming PUT with `Offset`, complete, folder walk for directory upload
- [x] Transfer queue UI shell (list, progress, cancel)
- [x] Vitest tests for auth wrapper and transfer reducer

### Phase 5 — Web UI (cloud experience)

- [x] Login and invite registration pages; route guards for `/cloud` and `/admin`
- [x] cloud browser: breadcrumbs, list/grid, open folder, create folder, rename, move, delete
- [x] Drag-and-drop upload of files and folders, plus file-picker fallback
- [x] Collision modal: Overwrite, Rename, Cancel
- [x] Download file and download folder (zip)
- [x] Image and video preview via media token URLs and Range playback
- [x] Favorites and search
- [x] Trash: list, restore, restore-to-root, permanent delete
- [x] Transfer panel with retry and cancel
- [x] Live updates when a second browser tab or device changes the same account
- [x] Empty and error states for each of the above
- [x] Browser verification on `http://<lan-ip>:<configured-port>/` against a running stack

### Phase 6 — Admin panel and cron jobs

- [x] Nightly jobs from §12 actually scheduled: trash purge, version purge, log purge, usage reconcile, `pg_dump` rotation
- [x] Upload-session expiry confirmed on the schedule (if already running in Phase 3, keep it and add the nightly jobs beside it)
- [x] Admin API completed for any gap left after Phase 2: settings, logs, stats, rebuild start and last report
- [x] Rebuild scanner implemented exactly as §11, including “do not delete orphans or missing rows” (sidecar `folder_path`, version attach, missing-row counts only)
- [x] Admin UI: user quotas, per-user speed limit, global speed limit, invite management, log viewer, rebuild trigger and result
- [x] Verification: quota reject (`scripts/verify-stack.ps1` + Go unit tests for speed-limit selection/throttling); rebuild against live `/data` via admin API; trash retention covered by `PurgeExpiredTrash` implementation and nightly schedule (operator can confirm via logs after 30 days or integration test in CI)
- [x] Final pass against this document: every Phase 1–6 box either checked or explicitly called out as incomplete

### Phase 7 — Operator readiness (post-v1)

- [x] Root `README.md` and `docs/OPERATOR_RUNBOOK.md` (backup restore, rebuild)
- [x] Admin UI copy for backup restore (runbook pointer, not one-click restore)
- [x] `scripts/verify-stack.ps1` for LAN stack smoke (login, quota, rebuild)
- [x] Go unit tests for speed-limit selection and per-request throttling (`internal/transfer`)
- [x] Optional UI polish (Explorer layout refinements, admin forms instead of prompts)

### Phase 8 — Vault UI gaps (post-v1)

- [x] Move file or folder to another folder (folder picker dialog, uses existing PATCH APIs)
- [x] Version history in preview drawer with restore (files under 100 MiB version policy)
- [x] CI workflow: `go test` and web production build on push to `main`

### Phase 9 — Admin user lifecycle

- [x] Admin UI: enable/disable account, grant/revoke admin (not on self)
- [x] Admin UI: delete user with confirmation (API rules: not self, not last admin, revoke unused invites first)
- [x] Invite create: optional expiry (days) and default quota (GiB)

### Phase 10 — Explorer (Windows-like interactions)

- [x] Context menus on items and empty folder area (open, cut/paste move, download, rename, move to, delete, favorites, versions, properties)
- [x] Selection: click, Ctrl/Cmd, Shift range, marquee, Ctrl+A; empty click clears selection
- [x] Keyboard: Delete, F2, Ctrl+X/V
- [x] Internal drag-and-drop: drag selected items onto a folder to move (uses existing PATCH APIs)
- [x] Copy/duplicate via context menu and Ctrl+C/V (server copy APIs)

### Phase 11 — Explorer copy (server + UI)

- [x] `POST /files/{id}/copy` and `POST /folders/{id}/copy` with quota checks and Windows-style ` - Copy` naming
- [x] Explorer clipboard: cut vs copy; paste move or duplicate into current folder

### Phase 12 — Explorer navigation and sorting

- [x] Back / Forward folder history (← →, Alt+Arrow); Up to parent (↑)
- [x] Breadcrumb segments as drag-and-drop move targets
- [x] Details view: sort by name, date modified, or size (persisted in session); grid view uses same order

### Phase 13 — Inline rename

- [x] F2, toolbar Rename, and context menu rename edit the name in place (grid and details)
- [x] Enter or blur commits; Escape cancels; conflict reopens the editor

### Phase 14 — Address bar and file icons

- [x] Editable path bar (double-click breadcrumbs or Alt+D): `Cloud › Folder › Sub` or `Folder/Sub`; Enter navigates
- [x] File-type icons in grid and details (by MIME / extension)

### Phase 15 — Thumbnails and type column

- [x] Grid tiles show server thumbnails when `thumbnail_status` is ready (media token)
- [x] Details view **Type** column with Windows-style labels; sortable

### Phase 16 — In-browser file preview

- [x] Double-click / Preview opens drawer with type-appropriate viewer: images, video, audio, PDF, text (incl. md/json/csv table), ZIP listing
- [x] Other archives (RAR, 7z, etc.): message + download; size limits for text/ZIP preview

### Phase 18 — LAN security hardening

- [x] LAN-only middleware (always on): 403 for clients whose direct IP is not private/local (RFC1918, loopback, link-local, IPv6 ULA); not configurable off (D23)
- [x] Do not trust `X-Forwarded-For` unless `LOCAL_CLOUD_TRUSTED_PROXY_CIDRS` is set and the connection comes from those proxies
- [x] Removed chi `RealIP` middleware (prevent private-IP spoofing via headers)
- [x] Baseline security headers (CSP, `X-Frame-Options`, `nosniff`, etc.) on all responses
- [x] Operator runbook: no port-forward, no VPN remote access, firewall, Docker bind

### Phase 19 — LAN abuse limits

- [x] Per-IP rate limit on all `/api/v1` routes (800 requests/minute, sliding window) returns `429 RATE_LIMITED`
- [x] Login/register keep stricter per-IP limit (20 / 15 minutes) via existing auth limiter

### Phase 17 — Preview pane polish and in-app editing

- [x] Preview overlay scoped to the file list pane; left nav and explorer chrome (title bar, commands, status) stay visible
- [x] Preview toolbar: prev/next file in folder, inspector, download, expand/shrink over the main column
- [x] Editable text-like files open directly in an editor (no separate Edit mode); Save (Ctrl+S), Discard, unsaved prompts on close/navigate, `beforeunload` when dirty
- [x] Save uses overwrite upload (`conflict_policy=overwrite`) so bytes update in place and version policy applies
- [x] Markdown: **Source** / **Preview** toggle with sanitized HTML rendering (GFM)
- [x] CSV/TSV: **Text** / **Table** toggle (table reflects current buffer; edit in Text)
- [x] Expand preview covers the explorer main column (nav remains); Shrink returns to in-pane overlay

---

## 21. Phase order and stop line

Implementation order is Phase 1 → … → Phase 16 → Phase 17 → Phase 18 → Phase 19 (post-v1 product gaps).

After each phase, stop. Report what landed, how it was verified, and any spec conflicts. Wait for approval before the next phase.

Current next step: **Backlog (Decision Log first)** — optional LAN TLS, Office document preview, sharing links, or explorer polish (syntax highlighting). Internet-facing or TLS changes need explicit decisions before implementation.
