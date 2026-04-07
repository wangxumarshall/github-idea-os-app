# Local Deployment Notes

This file is local-only for the current server.

Last updated: 2026-04-07

## Public Access

- Canonical site: `https://www.clawteam.io`
- Apex redirect: `https://clawteam.io` -> `https://www.clawteam.io`
- Public backend routes are proxied by Caddy on the same domain:
  - `/api/*`
  - `/auth/*`
  - `/ws`
  - `/health`

## Local Service Layout

- Backend listen address: `0.0.0.0:18080`
- Frontend listen address: `127.0.0.1:13000`
- Reverse proxy: `caddy` on `:80` and `:443`
- PostgreSQL: local database configured by `.env`

## Auth And Admin

- Admin email: `wangxumarshall@qq.com`
- Email verification login is working
- Super-admin password login is enabled through `.env`
- CLI app URL: `https://www.clawteam.io`
- Daemon server URL uses local loopback for reliability on the same host:
  - `http://127.0.0.1:18080`

## Workspace And Agent

- Workspace ID: `61993e8e-a5e4-48ab-8503-5b99c7107091`
- Daemon ID: `VM-0-3-ubuntu`
- Codex runtime ID: `6f5ad27e-17b8-43ce-8d91-cfaba7baf904`
- Agent name: `Local Codex`
- Agent ID: `95df723e-23cb-49dc-9980-393239f0975d`
- Agent status at setup: `idle`

## Managed Services

- `multica-server`
- `multica-web`
- `caddy`

## Important Files

- Repo env: `/home/ubuntu/multica/.env`
- Caddy config: `/etc/caddy/Caddyfile`
- Backend unit: `/etc/systemd/system/multica-server.service`
- Frontend unit: `/etc/systemd/system/multica-web.service`
- CLI config: `/home/ubuntu/.multica/config.json`
- Daemon log: `/home/ubuntu/.multica/daemon.log`

## Useful Commands

```bash
# Service status
systemctl is-active multica-server multica-web caddy

# Restart app and proxy
sudo systemctl restart caddy multica-server multica-web

# Check daemon
./server/bin/multica auth status
./server/bin/multica daemon status --output json
./server/bin/multica daemon logs -f

# Check agents
./server/bin/multica agent list --output json

# Local health checks
curl -sSf http://127.0.0.1:18080/health
curl -kI --resolve www.clawteam.io:443:127.0.0.1 https://www.clawteam.io
curl -kI --resolve clawteam.io:443:127.0.0.1 https://clawteam.io
```

## Current Caddy Routing

- `http://43.156.84.109` -> `https://www.clawteam.io{uri}`
- `https://clawteam.io` -> `https://www.clawteam.io{uri}`
- `https://www.clawteam.io`:
  - backend paths -> `127.0.0.1:18080`
  - everything else -> `127.0.0.1:13000`

## Known Follow-up

- `apps/web/app/layout.tsx` still has `metadataBase` pointing to `https://www.multica.ai`.
- If this self-hosted domain should be reflected in SEO/share metadata, update that file and rebuild the frontend.
