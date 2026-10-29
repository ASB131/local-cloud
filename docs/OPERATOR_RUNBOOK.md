# Local Cloud — Operator runbook

LAN-only deployment. Default UI: `http://<host-lan-ip>:8090/` (see `deploy/.env` for `LOCAL_CLOUD_HOST_PORT`).

**Access model:** devices must be on your **home LAN** (same router/Wi‑Fi or Ethernet). This product is **not** for use over the internet, port-forwarding, or VPN from another location. If you need files while away from home, copy them by other means; do not expose Local Cloud remotely.

## Security hardening (home network)

Local Cloud is designed for **LAN-only** use. No application can make a port-forward “safe”; use **layers**:

1. **Do not port-forward** TCP 8090 (or your chosen port) on the router.
2. **Do not use VPN/mesh** (Tailscale, WireGuard, etc.) as a way to reach Local Cloud from outside the house. That is outside product intent and increases risk if misconfigured.
3. **Host firewall:** allow inbound 8090 only from private ranges (e.g. Windows “Private network” profile, or `ufw allow from 192.168.0.0/16 to any port 8090`).
4. **Docker bind (optional):** in `deploy/docker-compose.yml`, bind `ports` to your server’s LAN IP instead of `0.0.0.0` (see comment in that file).
5. **App LAN-only filter (always on):** the API returns **403** to clients whose **direct** TCP source is not a private/local IP (e.g. traffic from the internet via port-forward). This cannot be disabled.
6. **Secrets:** strong `LOCAL_CLOUD_JWT_SECRET` and Postgres password; change default **admin** password after first login; use **invite-only** registration.
7. **No trusted proxy by default:** leave `LOCAL_CLOUD_TRUSTED_PROXY_CIDRS` empty unless you run a reverse proxy **on the LAN only**. Otherwise `X-Forwarded-For` cannot be used to spoof a private IP.

Default is **HTTP on the LAN**. Optional **TLS** (`LOCAL_CLOUD_TLS_CERT` / `LOCAL_CLOUD_TLS_KEY`, see `deploy/.env.example`) encrypts traffic on the wire and helps against passive sniffing on Wi‑Fi; it does **not** replace the LAN-only filter or fix port-forwarding. A guest VLAN still helps isolate untrusted devices.

**Office preview:** LibreOffice converts documents to PDF in the API container (async). Optional **OnlyOffice**: `docker compose --profile onlyoffice up -d`, set `LOCAL_CLOUD_ONLYOFFICE_PUBLIC_URL` to the Document Server URL browsers can reach (e.g. `http://192.168.x.x:8082`). When set, the UI prefers OnlyOffice for Word/Excel/PowerPoint preview.

## Daily operations

- **Users and invites:** Admin → Users / Invites. Invite codes are shown once at creation.
- **Quotas and speed:** Admin → per-user actions, or global speed under Admin settings.
- **Rebuild from disk:** Admin → **Run rebuild** when Postgres metadata is missing or corrupt but `/data/users` still has object trees and `meta.json` sidecars. Rebuild does **not** drop the database or delete orphan blobs; read the JSON report for counts.
- **Logs:** Admin → Recent logs (also stored in Postgres, purged after 30 days).

## Backups (preferred recovery)

Nightly `pg_dump -Fc` files land in `/data/backups/` inside the `localcloud-api` container (retain 7).

**Restore a backup (manual, destructive to current DB):**

1. Stop writes: `docker compose -f deploy/docker-compose.yml stop localcloud-api`
2. Copy the chosen `.dump` file from the volume (or `docker compose exec localcloud-api ls /data/backups`).
3. Restore into Postgres (example — adjust password and dump name):

   ```bash
   docker compose -f deploy/docker-compose.yml exec -T postgres \
     pg_restore -U localcloud -d localcloud --clean --if-exists < backup.dump
   ```

   For a custom-format dump file on the host, pipe it:

   ```bash
   docker compose -f deploy/docker-compose.yml exec -T postgres \
     pg_restore -U localcloud -d localcloud --clean --if-exists < /path/on/host/backup.dump
   ```

4. Start API: `docker compose -f deploy/docker-compose.yml start localcloud-api`
5. Verify `/readyz` and log in as admin.

If the database is empty but disk data remains, run migrations then **Rebuild database** instead of restore.

## Maintenance jobs

All run inside `localcloud-api` (UTC): trash purge 02:15, version purge 02:30, log purge 02:45, upload expiry every 15 minutes, usage reconcile 03:00, backup 03:15.

## Verification

From the repo root (PowerShell):

```powershell
.\scripts\verify-stack.ps1 -BaseUrl http://localhost:8090
```

Uses admin credentials from `deploy/.env` when present, otherwise `admin` / `admin`.
