# discord-modguard

Discord moderation bot + web admin panel, shipped as Docker images.

**Slash commands · auto-moderation · warnings with auto-escalation · REST API · stats dashboard.**

Source code: [github.com/yuliitezarygml/discord-modguard](https://github.com/yuliitezarygml/discord-modguard)

---

## Images

| Image                                           | Description                  |
| ----------------------------------------------- | ---------------------------- |
| `yuliitezar/discord-modguard-bot:latest`        | Go bot + embedded REST API   |
| `yuliitezar/discord-modguard-frontend:latest`   | Next.js admin panel          |

Both images are multi-arch (`linux/amd64`, `linux/arm64`). Pair with `postgres:16-alpine` for storage.

---

## Quick start

### 1. Create a folder and env file

```bash
mkdir discord-modguard && cd discord-modguard
curl -O https://raw.githubusercontent.com/yuliitezarygml/discord-modguard/main/.env.example
curl -O https://raw.githubusercontent.com/yuliitezarygml/discord-modguard/main/docker-compose.prod.yml
mv .env.example .env
```

### 2. Fill in `.env`

```env
DISCORD_TOKEN=<your_bot_token>
DISCORD_APP_ID=<your_application_id>
DB_PASSWORD=<random_password>
JWT_SECRET=<long_random_string>
FRONTEND_URL=http://localhost:3000
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Generate secure secrets:
```bash
openssl rand -hex 32     # JWT_SECRET
openssl rand -hex 16     # DB_PASSWORD
```

### 3. Launch

```bash
docker compose -f docker-compose.prod.yml up -d
```

### 4. Open

- Admin panel → http://localhost:3000
- API health → http://localhost:8080/health

The **first account** you register in the panel becomes `admin`.

---

## Creating a Discord bot

1. https://discord.com/developers/applications → **New Application**
2. **Bot** tab → copy token into `DISCORD_TOKEN`
3. **General** tab → copy Application ID into `DISCORD_APP_ID`
4. Enable **Privileged Gateway Intents**: *Server Members* + *Message Content*
5. Invite (replace `<APP_ID>`):

```
https://discord.com/api/oauth2/authorize?client_id=<APP_ID>&scope=bot+applications.commands&permissions=1099511627782
```

---

## Slash commands

`/ban`, `/kick`, `/mute`, `/unmute`, `/warn`, `/warnings`, `/clearwarnings` — with per-command Discord permission gates.

Warning escalation auto-mutes at 3 active warnings and auto-bans at 5 (configurable per guild).

---

## Ports

- `3000` — admin panel (Next.js)
- `8080` — REST API
- `5432` — PostgreSQL (only needed if you want external DB access)

---

## Update

```bash
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

---

## Tags

- `latest` — stable builds from `main`
- `vX.Y.Z` — tagged releases

Full docs, REST API reference, and contribution guide: [github.com/yuliitezarygml/discord-modguard](https://github.com/yuliitezarygml/discord-modguard)
