# discord-modguard

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?logo=next.js)](https://nextjs.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](https://hub.docker.com)
[![License](https://img.shields.io/badge/license-MIT-green)](#license)

Discord moderation bot + web admin panel. Slash-commands, auto-moderation, warnings with auto-escalation, REST API, statistics dashboard. Scales comfortably to 10–20 guilds.

- **Bot & API** — Go 1.22, [discordgo](https://github.com/bwmarrin/discordgo), [gin](https://github.com/gin-gonic/gin), [pgx](https://github.com/jackc/pgx)
- **Admin panel** — Next.js 14 (App Router), TypeScript, Tailwind CSS, Recharts
- **Storage** — PostgreSQL 16
- **Deployment** — Docker Compose (dev from source / prod from Docker Hub)

---

## Table of contents

- [Features](#features)
- [Quick start (Docker Hub)](#quick-start-docker-hub)
- [Quick start (from source / GitHub)](#quick-start-from-source--github)
- [Creating a Discord bot](#creating-a-discord-bot)
- [Environment variables](#environment-variables)
- [Slash commands](#slash-commands)
- [Auto-moderation rules](#auto-moderation-rules)
- [REST API](#rest-api)
- [Development](#development)
- [Publishing Docker images](#publishing-docker-images)
- [Project structure](#project-structure)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Features

- **Slash commands** — `/ban`, `/kick`, `/mute`, `/unmute`, `/warn`, `/warnings`, `/clearwarnings`
- **Auto-moderation** — word filter (plain + regex), spam detection, raid protection, custom rules
- **Warning escalation** — configurable thresholds auto-escalate to mute / ban
- **Temporary punishments** — mutes and bans expire automatically (background ticker)
- **Admin panel** — guild list with server icon/banner, paginated logs, rule CRUD, time-series stats, settings
- **REST API** — JWT auth, role-based access (`admin` / `moderator`), CORS, rate-limiting friendly
- **Embedded migrations** — no external migration tool required

---

## Quick start (Docker Hub)

The easiest way to run `discord-modguard` — no source checkout, just pull the prebuilt images.

### 1. Create a project folder and env file

```bash
mkdir discord-modguard && cd discord-modguard
curl -O https://raw.githubusercontent.com/yuliitezarygml/discord-modguard/main/.env.example
curl -O https://raw.githubusercontent.com/yuliitezarygml/discord-modguard/main/docker-compose.prod.yml
mv .env.example .env
```

### 2. Edit `.env`

At minimum set:

```env
DISCORD_TOKEN=<your_bot_token>
DISCORD_APP_ID=<your_application_id>
DB_PASSWORD=<any_random_password>
JWT_SECRET=<long_random_string>
```

### 3. Launch

```bash
docker compose -f docker-compose.prod.yml up -d
```

### 4. Open the panel

- Admin panel → http://localhost:3000
- API health → http://localhost:8080/health

The **first account** you register becomes `admin`; subsequent accounts become `moderator`.

Images are hosted on **GitHub Container Registry**: [ghcr.io/yuliitezarygml/discord-modguard-bot](https://github.com/yuliitezarygml/discord-modguard/pkgs/container/discord-modguard-bot) and [...-frontend](https://github.com/yuliitezarygml/discord-modguard/pkgs/container/discord-modguard-frontend).

---

## Quick start (from source / GitHub)

Requires Docker + Docker Compose. Build locally instead of pulling.

```bash
git clone https://github.com/yuliitezarygml/discord-modguard.git
cd discord-modguard
cp .env.example .env
# edit .env — fill in DISCORD_TOKEN, DISCORD_APP_ID, DB_PASSWORD, JWT_SECRET

docker compose up -d --build
```

Then open http://localhost:3000.

To tail logs:

```bash
docker compose logs -f bot
docker compose logs -f frontend
```

---

## Creating a Discord bot

1. Go to https://discord.com/developers/applications and **New Application**.
2. On the **Bot** tab: *Reset Token* → copy it into `.env` as `DISCORD_TOKEN`.
3. On the **General Information** tab: copy *Application ID* into `.env` as `DISCORD_APP_ID`.
4. On the **Bot** tab, enable **Privileged Gateway Intents**:
   - ✅ Server Members Intent
   - ✅ Message Content Intent
5. Invite the bot to your server using the OAuth2 URL generator (`bot` + `applications.commands` scopes, permissions: **Ban Members**, **Kick Members**, **Moderate Members**, **Manage Messages**, **View Channels**, **Send Messages**).

Example invite URL (replace `<APP_ID>`):
```
https://discord.com/api/oauth2/authorize?client_id=<APP_ID>&scope=bot+applications.commands&permissions=1099511627782
```

---

## Environment variables

| Variable              | Required | Description                                          |
| --------------------- | :------: | ---------------------------------------------------- |
| `DISCORD_TOKEN`       | ✅       | Bot token from Discord developer portal              |
| `DISCORD_APP_ID`      |          | Application ID (resolved from session if omitted)    |
| `DB_PASSWORD`         | ✅       | Postgres password                                    |
| `DATABASE_URL`        |          | Full connection string (default uses `DB_PASSWORD`)  |
| `JWT_SECRET`          | ✅       | Long random string used to sign JWTs                 |
| `API_PORT`            |          | API port, default `8080`                             |
| `FRONTEND_URL`        |          | Allowed CORS origin, default `http://localhost:3000` |
| `NEXT_PUBLIC_API_URL` |          | API base URL baked into the frontend at build time   |

Generate secure secrets:
```bash
openssl rand -hex 32     # JWT_SECRET
openssl rand -hex 16     # DB_PASSWORD
```

---

## Slash commands

| Command          | Arguments                                   | Required Discord permission |
| ---------------- | ------------------------------------------- | --------------------------- |
| `/ban`           | `user`, `reason?`, `duration?` (`1h`, `7d`) | Ban Members                 |
| `/kick`          | `user`, `reason?`                           | Kick Members                |
| `/mute`          | `user`, `duration`, `reason?`               | Moderate Members            |
| `/unmute`        | `user`                                      | Moderate Members            |
| `/warn`          | `user`, `reason`                            | Moderate Members            |
| `/warnings`      | `user`                                      | Moderate Members            |
| `/clearwarnings` | `user`                                      | Administrator               |

**Warning escalation:** if a user accumulates `warn_mute_threshold` active warnings (default 3), they are auto-muted for 1 hour; at `warn_ban_threshold` (default 5) — auto-banned. Configure per-guild in *Settings*.

Commands are registered globally on bot startup via `ApplicationCommandBulkOverwrite`.

---

## Auto-moderation rules

Stored in the `auto_mod_rules` table, managed from **Admin panel → Auto-mod**.

### word_filter
```json
{
  "words": ["badword1", "badword2"],
  "patterns": ["\\b(regex|here)\\b"],
  "action": "delete"
}
```

### spam_detection
```json
{
  "limit": 5,
  "window_seconds": 10,
  "action": "mute"
}
```

### raid_protection
```json
{
  "limit": 10,
  "window_seconds": 30,
  "alert_channel_id": "123456789012345678"
}
```

### custom
Reserved for user-defined logic — same `config_json` shape, evaluated by your own extension.

**Actions:** `delete` (remove message), `warn` (delete + add warning), `mute` (delete + 10-minute timeout), `ban`.

---

## REST API

Base URL: `http://localhost:8080/api`

### Authentication
| Method | Path                  | Body / Params                        | Notes                               |
| ------ | --------------------- | ------------------------------------ | ----------------------------------- |
| POST   | `/auth/register`      | `{ email, password }`                | First user becomes `admin`          |
| POST   | `/auth/login`         | `{ email, password }`                | Returns `{ user, token }`           |
| POST   | `/auth/logout`        | —                                    | Client-side token drop              |
| GET    | `/me`                 | —                                    | Current user (requires Bearer)      |

### Guilds
| Method | Path                               | Role  | Description                     |
| ------ | ---------------------------------- | ----- | ------------------------------- |
| GET    | `/guilds`                          | any   | All guilds (with icon/banner)   |
| GET    | `/guilds/:id`                      | any   | Guild detail                    |
| PUT    | `/guilds/:id/settings`             | admin | Update settings JSON            |

### Moderation
| Method | Path                               | Role  | Description                     |
| ------ | ---------------------------------- | ----- | ------------------------------- |
| GET    | `/guilds/:id/logs`                 | any   | `?page=1&limit=50&action_type=` |
| GET    | `/guilds/:id/warnings?user_id=…`   | any   | Warnings for a user             |
| POST   | `/guilds/:id/ban`                  | admin | `{ user_id, reason, duration }` |
| DELETE | `/guilds/:id/ban/:userId`          | admin | Unban                           |

### Auto-moderation
| Method | Path                               | Role  |
| ------ | ---------------------------------- | ----- |
| GET    | `/guilds/:id/automod`              | any   |
| POST   | `/guilds/:id/automod`              | admin |
| PUT    | `/guilds/:id/automod/:ruleId`      | admin |
| DELETE | `/guilds/:id/automod/:ruleId`      | admin |

### Statistics
| Method | Path                               | Params                         |
| ------ | ---------------------------------- | ------------------------------ |
| GET    | `/guilds/:id/stats`                | `period=7d\|30d\|90d\|all`     |

All protected endpoints require `Authorization: Bearer <JWT>`.

---

## Development

### Backend (Go)

```bash
cd bot
go mod tidy
export DISCORD_TOKEN=...
export DATABASE_URL="postgres://user:pass@localhost:5432/discord_bot?sslmode=disable"
export JWT_SECRET=...
go run ./cmd/bot
```

Run migrations only (they also run on normal startup):
```bash
go run ./cmd/bot migrate
```

Tests / vet:
```bash
go vet ./...
go test ./...
```

### Frontend (Next.js)

```bash
cd frontend
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
```

Open http://localhost:3000. Hot-reload works out of the box.

---

## Publishing Docker images

Images are published automatically to **GitHub Container Registry** (`ghcr.io`) on every push to `main` and every `v*.*.*` tag — see [.github/workflows/publish.yml](.github/workflows/publish.yml). No manual action required.

Published images:

| Image                                                       | Tags                                        |
| ----------------------------------------------------------- | ------------------------------------------- |
| `ghcr.io/yuliitezarygml/discord-modguard-bot`               | `latest`, `main`, `vX.Y.Z`, `sha-<commit>`  |
| `ghcr.io/yuliitezarygml/discord-modguard-frontend`          | same                                        |

**To publish a versioned release**, push a semver tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds both images multi-arch (`linux/amd64` + `linux/arm64`) and pushes them with `latest`, `v0.1.0`, and `0.1` tags.

**To make images public**, after the first successful publish go to
`https://github.com/yuliitezarygml?tab=packages` → click a package →
*Package settings* → *Change visibility* → **Public**.

**Manual build** (if you need to push to Docker Hub or a private registry):

```bash
docker login
docker buildx build --platform linux/amd64,linux/arm64 \
  -t <your-registry>/discord-modguard-bot:latest --push ./bot
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg NEXT_PUBLIC_API_URL=http://your-host:8080 \
  -t <your-registry>/discord-modguard-frontend:latest --push ./frontend
```

---

## Project structure

```
discord-modguard/
├── bot/
│   ├── cmd/bot/main.go             # entry point
│   ├── internal/
│   │   ├── api/                    # REST server (gin)
│   │   ├── automod/                # auto-mod engine
│   │   ├── bot/                    # Discord bot + slash commands
│   │   ├── config/                 # env config
│   │   ├── database/
│   │   │   ├── database.go
│   │   │   └── migrations/         # embedded SQL
│   │   ├── models/
│   │   └── store/                  # data access layer
│   └── Dockerfile
├── frontend/
│   ├── app/                        # Next.js App Router pages
│   ├── components/
│   ├── lib/api.ts                  # fetch wrapper + types
│   └── Dockerfile
├── docker-compose.yml              # build from source (dev)
├── docker-compose.prod.yml         # pull from Docker Hub (prod)
├── .env.example
└── README.md
```

---

## Troubleshooting

**The bot connects but slash commands don't appear.** Discord can take up to an hour to propagate global commands. Kick the bot and re-invite, or switch to guild-scoped registration for development.

**`panic: DISCORD_TOKEN is required`** on startup. You didn't mount the `.env` file. With `docker compose` it's loaded automatically; with a bare `docker run` use `--env-file .env`.

**Login fails with "invalid credentials".** Passwords are bcrypt-hashed — re-register if you forgot it (or truncate the `users` table).

**CORS errors in the browser.** `FRONTEND_URL` must match exactly the origin you load the panel from (scheme + host + port). Default is `http://localhost:3000`.

**Frontend shows 0 guilds.** The bot must actually be *in* a guild. Invite it via the OAuth URL above and wait a few seconds for the `GUILD_CREATE` event.

---

## License

MIT. See [LICENSE](LICENSE) if included, otherwise:

> Copyright (c) 2026 discord-modguard contributors. Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files...
