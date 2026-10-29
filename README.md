# Local Cloud

LAN-only file storage: Go API, React UI, PostgreSQL, Docker Compose. Use only from devices on your home network — not over the internet, port-forwarding, or VPN from elsewhere.

## Build and run

```powershell
copy deploy\.env.example deploy\.env
# edit deploy\.env — passwords and LOCAL_CLOUD_JWT_SECRET
cd deploy
docker compose up -d --build
```

Open `http://<this-machine-lan-ip>:8090/`. Default admin user is `admin` / `admin` if you leave the example values.

## Pull a release image

Use the root `docker-compose.yml` with a `.env` copied from `deploy/.env.example`. Image: `ghcr.io/asb131/localcloud:v0.1.0-pre.1` (tag matches the release).

More detail: `docs/PROJECT_SPEC.md`, `docs/OPERATOR_RUNBOOK.md`.
