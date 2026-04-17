# Discord Moderation Bot + Admin Panel

A production-ready Discord moderation system with a web admin panel.

- **Bot & API** — Go 1.22, [discordgo](https://github.com/bwmarrin/discordgo), [gin](https://github.com/gin-gonic/gin), [pgx](https://github.com/jackc/pgx)
- **Admin panel** — Next.js 14 (App Router), TypeScript, Tailwind CSS, Recharts
- **Storage** — PostgreSQL 16
- **Deployment** — Docker Compose

## Features

- Slash commands: `/ban`, `/kick`, `/mute`, `/unmute`, `/warn`, `/warnings`, `/clearwarnings`
- Auto-moderation engine: word filter, spam detection, raid protection, custom rules
- Warning escalation (configurable thresholds → auto-mute / auto-ban)
- Temporary punishments with automatic expiry
- REST API with JWT auth (first registered user becomes admin)
- Admin panel: guild list, moderation logs (filter + paginate), auto-mod rule CRUD, stats with time-series chart, per-guild settings

## Quick start

Requires: Docker, Docker Compose, a Discord bot token ([create one](https://discord.com/developers/applications)).

```bash
git clone <this-repo> discordbot
cd discordbot
cp .env.example .env
# edit .env — set DISCORD_TOKEN, DB_PASSWORD, JWT_SECRET
docker compose up -d --build
```

- Admin panel: http://localhost:3000
- API:         http://localhost:8080 (health: `/health`)

The first account you register becomes `admin`. Invite the bot to your Discord server with the `bot` + `applications.commands` scopes and permissions to ban/kick/timeout.

## Environment variables

| Variable              | Description                                          |
| --------------------- | ---------------------------------------------------- |
| `DISCORD_TOKEN`       | Bot token from Discord developer portal              |
| `DISCORD_APP_ID`      | Application ID (optional — resolved from session)    |
| `DB_PASSWORD`         | Postgres password                                    |
| `JWT_SECRET`          | Long random string used to sign JWTs                 |
| `FRONTEND_URL`        | Allowed CORS origin (default `http://localhost:3000`)|
| `NEXT_PUBLIC_API_URL` | API base URL used by the frontend at build time      |

## Local development (without Docker)

### Bot + API

```bash
cd bot
go mod tidy
DISCORD_TOKEN=... DATABASE_URL=postgres://... JWT_SECRET=... \
  go run ./cmd/bot
```

Run migrations only: `go run ./cmd/bot migrate` — migrations are embedded and
also applied automatically on startup.

### Frontend

```bash
cd frontend
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
```

## REST API (summary)

See [docs/superpowers/specs/2026-04-17-discord-moderation-bot-design.md](docs/superpowers/specs/2026-04-17-discord-moderation-bot-design.md) for the full design.

| Method | Path                                 | Notes                          |
| ------ | ------------------------------------ | ------------------------------ |
| POST   | `/api/auth/register`                 | First user becomes admin       |
| POST   | `/api/auth/login`                    | Returns `{ user, token }`      |
| GET    | `/api/me`                            | Current user                   |
| GET    | `/api/guilds`                        | All guilds the bot is in       |
| GET    | `/api/guilds/:id`                    | Guild detail                   |
| PUT    | `/api/guilds/:id/settings`           | Admin only                     |
| GET    | `/api/guilds/:id/logs`               | Paginated + filterable         |
| GET    | `/api/guilds/:id/warnings?user_id=…` | User warnings                  |
| POST   | `/api/guilds/:id/ban`                | Admin only                     |
| DELETE | `/api/guilds/:id/ban/:userId`        | Admin only                     |
| GET    | `/api/guilds/:id/automod`            | List rules                     |
| POST   | `/api/guilds/:id/automod`            | Admin only — create rule       |
| PUT    | `/api/guilds/:id/automod/:ruleId`    | Admin only — update rule       |
| DELETE | `/api/guilds/:id/automod/:ruleId`    | Admin only — delete rule       |
| GET    | `/api/guilds/:id/stats?period=30d`   | Time series + aggregates       |

## Auto-mod rule configs

**word_filter**
```json
{ "words": ["badword"], "patterns": ["^.*$"], "action": "delete" }
```

**spam_detection**
```json
{ "limit": 5, "window_seconds": 10, "action": "mute" }
```

**raid_protection**
```json
{ "limit": 10, "window_seconds": 30, "alert_channel_id": "123456789012345678" }
```

Actions: `delete`, `warn`, `mute`, `ban`.

## Guild settings

Stored as JSON in `guilds.settings_json`. Recognised keys:

- `warn_mute_threshold` (number, default `3`) — auto-mute at this many active warnings
- `warn_ban_threshold`  (number, default `5`) — auto-ban at this many active warnings

## Project structure

```
discordbot/
├── bot/
│   ├── cmd/bot/main.go          # entry point
│   ├── internal/
│   │   ├── api/                 # REST server
│   │   ├── automod/             # auto-mod engine
│   │   ├── bot/                 # Discord bot + commands
│   │   ├── config/
│   │   ├── database/migrations/ # embedded SQL migrations
│   │   ├── models/
│   │   └── store/               # data access
│   └── Dockerfile
├── frontend/
│   ├── app/                     # Next.js App Router pages
│   ├── components/
│   ├── lib/api.ts               # fetch wrapper + types
│   └── Dockerfile
├── docker-compose.yml
├── .env.example
└── README.md
```

## License

MIT.
