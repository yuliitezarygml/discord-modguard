# Discord Moderation Bot with Admin Panel - Design Specification

**Date:** 2026-04-17  
**Project:** Discord moderation bot (Go) + Admin panel (Next.js)  
**Target Scale:** 10-20 Discord servers  
**Deployment:** Docker containers

---

## 1. Overview

A comprehensive Discord moderation system consisting of:
- **Discord Bot (Go)** - handles moderation commands, auto-moderation, and events
- **REST API (Go)** - embedded in bot, provides endpoints for admin panel
- **Admin Panel (Next.js)** - web interface for configuration and monitoring
- **PostgreSQL** - persistent storage for all data
- **Docker Compose** - orchestration and deployment

### Architecture Approach

Monolithic architecture with bot and API in a single Go process. This provides:
- Simplified development and deployment
- Sufficient performance for 10-20 servers
- Easy debugging and maintenance
- Future migration path to microservices if needed

---

## 2. System Architecture

### Components

```
┌─────────────────┐
│   Admin Panel   │ (Next.js, port 3000)
│   (Next.js)     │
└────────┬────────┘
         │ HTTP/JSON
         ▼
┌─────────────────┐
│   REST API      │ (Go, port 8080)
│   (embedded)    │
└────────┬────────┘
         │
┌────────┴────────┐
│  Discord Bot    │ (Go)
│  (discordgo)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   PostgreSQL    │ (port 5432)
└─────────────────┘
         ▲
         │
    Discord API
```

### Communication Flow

- **Next.js → REST API**: HTTP requests with JWT authentication
- **Bot → PostgreSQL**: Direct connection via pgx driver
- **API → PostgreSQL**: Shared connection pool with bot
- **Bot → Discord**: WebSocket connection via discordgo library

---

## 3. Database Schema

### Core Tables

**users** - Admin panel users
```sql
- id (uuid, primary key)
- email (varchar, unique)
- password_hash (varchar)
- role (enum: admin, moderator)
- created_at (timestamp)
- updated_at (timestamp)
```

**guilds** - Discord servers
```sql
- id (bigint, primary key) -- Discord guild ID
- name (varchar)
- settings_json (jsonb) -- flexible config storage
- added_at (timestamp)
- updated_at (timestamp)
```

**moderation_logs** - All moderation actions
```sql
- id (uuid, primary key)
- guild_id (bigint, foreign key)
- moderator_id (bigint) -- Discord user ID
- action_type (enum: ban, kick, mute, warn, unmute, unban)
- target_user_id (bigint)
- target_username (varchar)
- reason (text)
- duration (interval, nullable) -- for temporary actions
- timestamp (timestamp)
```

**auto_mod_rules** - Auto-moderation configuration
```sql
- id (uuid, primary key)
- guild_id (bigint, foreign key)
- rule_type (enum: word_filter, spam_detection, raid_protection, custom)
- config_json (jsonb) -- rule-specific configuration
- enabled (boolean)
- created_at (timestamp)
- updated_at (timestamp)
```

**warnings** - User warnings tracking
```sql
- id (uuid, primary key)
- guild_id (bigint, foreign key)
- user_id (bigint)
- username (varchar)
- reason (text)
- moderator_id (bigint)
- moderator_name (varchar)
- timestamp (timestamp)
- active (boolean) -- can be cleared/expired
```

**banned_words** - Word filter entries
```sql
- id (uuid, primary key)
- guild_id (bigint, foreign key)
- word (varchar)
- is_regex (boolean)
- created_at (timestamp)
```

**punishments** - Active punishments
```sql
- id (uuid, primary key)
- guild_id (bigint, foreign key)
- user_id (bigint)
- type (enum: mute, ban, timeout)
- reason (text)
- duration (interval, nullable)
- expires_at (timestamp, nullable)
- created_at (timestamp)
- active (boolean)
```

### Indexes

- `moderation_logs(guild_id, timestamp DESC)` - log queries
- `warnings(guild_id, user_id)` - user warning lookups
- `punishments(guild_id, user_id, active)` - active punishment checks
- `auto_mod_rules(guild_id, enabled)` - rule lookups

---

## 4. Discord Bot (Go)

### Technology Stack

- **discordgo** - Discord API library
- **gin** or **fiber** - REST API framework (recommend gin for simplicity)
- **pgx** - PostgreSQL driver
- **golang-jwt/jwt** - JWT authentication
- **go-playground/validator** - input validation
- **zap** or **logrus** - structured logging

### Core Modules

#### 4.1 Commands Handler

Slash commands implementation:
- `/ban <user> <reason> [duration]` - ban user (permanent or temporary)
- `/kick <user> <reason>` - kick user
- `/mute <user> <duration> <reason>` - mute user
- `/unmute <user>` - unmute user
- `/warn <user> <reason>` - warn user
- `/warnings <user>` - view user warnings
- `/clearwarnings <user>` - clear user warnings
- `/config` - view/edit server configuration

**Why:** Slash commands provide better UX and are Discord's recommended approach.

**How to apply:** Use discordgo's interaction handlers, validate permissions before execution, log all actions to database.

#### 4.2 Event Listener

Monitors Discord events:
- `MessageCreate` - check messages against auto-mod rules
- `GuildMemberAdd` - raid protection, welcome messages
- `GuildMemberRemove` - log departures
- `GuildBanAdd/Remove` - sync ban state with database

**Why:** Real-time event processing enables auto-moderation and audit logging.

**How to apply:** Register event handlers in discordgo, process asynchronously where possible to avoid blocking.

#### 4.3 Auto-Moderation Engine

**Word Filter:**
- Check messages against banned_words table
- Support both exact match and regex patterns
- Configurable actions: delete, warn, mute, ban

**Spam Detection:**
- Track message frequency per user (in-memory cache with TTL)
- Configurable thresholds: N messages in X seconds
- Progressive punishment: warn → mute → kick → ban

**Raid Protection:**
- Detect mass joins (N users in X seconds)
- Auto-actions: enable verification, kick new joins, lock channels
- Alert moderators via Discord channel

**Custom Triggers:**
- User-defined rules stored in auto_mod_rules
- Condition matching (message content, user roles, channel)
- Configurable actions

**Why:** Automated moderation reduces moderator workload and provides 24/7 protection.

**How to apply:** Process events through rule engine, cache rules in memory, reload on config changes.

#### 4.4 Punishment System

**Escalation Logic:**
- Track warning count per user
- Configurable thresholds (default: 3 warns = mute, 5 warns = ban)
- Temporary punishments with auto-expiry
- Background job checks expires_at and removes expired punishments

**Why:** Graduated punishment system encourages behavior improvement while maintaining order.

**How to apply:** Check warning count before applying punishment, schedule expiry checks via ticker goroutine.

#### 4.5 REST API Server

Embedded HTTP server for admin panel:
- JWT-based authentication
- CORS configured for frontend URL only
- Rate limiting per endpoint
- Request validation middleware
- Error handling middleware

**Why:** Embedded API simplifies deployment and shares database connection pool with bot.

**How to apply:** Start HTTP server in separate goroutine, use middleware chain for auth/validation/logging.

---

## 5. REST API Endpoints

### Authentication

**POST /api/auth/register**
- Body: `{ email, password }`
- Returns: `{ user, token }`
- Validation: email format, password strength (min 8 chars)

**POST /api/auth/login**
- Body: `{ email, password }`
- Returns: `{ user, token }`
- Token expiry: 24 hours

**POST /api/auth/logout**
- Headers: `Authorization: Bearer <token>`
- Returns: `{ success: true }`
- Invalidates token (optional: maintain blacklist)

### Guild Management

**GET /api/guilds**
- Headers: `Authorization: Bearer <token>`
- Returns: `[{ id, name, memberCount, settings }]`
- Lists all guilds the bot is in

**GET /api/guilds/:id**
- Returns: `{ id, name, memberCount, settings, stats }`
- Stats: total bans, warns, active rules

**PUT /api/guilds/:id/settings**
- Body: `{ settings_json }`
- Returns: `{ guild }`
- Updates guild configuration

### Moderation

**GET /api/guilds/:id/logs**
- Query params: `?page=1&limit=50&action_type=ban&moderator_id=123`
- Returns: `{ logs: [], total, page, limit }`
- Pagination and filtering support

**GET /api/guilds/:id/warnings**
- Query params: `?user_id=123`
- Returns: `[{ id, user_id, reason, moderator, timestamp }]`

**POST /api/guilds/:id/ban**
- Body: `{ user_id, reason, duration? }`
- Returns: `{ success: true, log }`
- Executes ban via Discord API

**DELETE /api/guilds/:id/ban/:userId**
- Returns: `{ success: true }`
- Unbans user

### Auto-Moderation

**GET /api/guilds/:id/automod**
- Returns: `[{ id, rule_type, config, enabled }]`

**POST /api/guilds/:id/automod**
- Body: `{ rule_type, config, enabled }`
- Returns: `{ rule }`
- Creates new auto-mod rule

**PUT /api/guilds/:id/automod/:ruleId**
- Body: `{ config?, enabled? }`
- Returns: `{ rule }`

**DELETE /api/guilds/:id/automod/:ruleId**
- Returns: `{ success: true }`

### Statistics

**GET /api/guilds/:id/stats**
- Query params: `?period=7d` (7d, 30d, 90d, all)
- Returns: `{ totalBans, totalWarns, totalKicks, activeRules, timeline: [] }`
- Timeline: daily aggregated stats for charts

---

## 6. Admin Panel (Next.js)

### Technology Stack

- **Next.js 14+** - App Router
- **TypeScript** - type safety
- **Tailwind CSS** - utility-first styling
- **shadcn/ui** - pre-built components (Button, Table, Dialog, etc.)
- **TanStack Query (React Query)** - API state management and caching
- **Zustand** - client-side state (auth, UI state)
- **Recharts** - statistics visualization
- **React Hook Form** - form handling
- **Zod** - schema validation

### Pages Structure

```
/app
  /login - authentication page
  /register - registration page
  /dashboard - main dashboard (guild list, overview)
  /guilds/[id]
    /page.tsx - guild overview
    /logs - moderation logs table
    /automod - auto-mod rules management
    /settings - guild configuration
    /stats - statistics and charts
  /layout.tsx - root layout with auth check
```

### Key Features

#### 6.1 Authentication Flow

- Login/register forms with validation
- JWT stored in httpOnly cookie or localStorage
- Auth state managed via Zustand
- Protected routes with middleware redirect
- Auto-refresh token before expiry

**Why:** Secure authentication prevents unauthorized access to moderation tools.

**How to apply:** Use Next.js middleware for route protection, axios interceptors for token refresh.

#### 6.2 Guild Dashboard

- Card grid showing all guilds
- Quick stats per guild (members, active rules, recent actions)
- Search and filter guilds
- Click to navigate to guild detail

#### 6.3 Moderation Logs

- Paginated table with columns: timestamp, action, moderator, target, reason
- Filters: action type, moderator, date range
- Search by username
- Export to CSV (optional)

**Why:** Comprehensive audit trail for accountability and analysis.

**How to apply:** Use shadcn/ui Table component, React Query for pagination, debounced search input.

#### 6.4 Auto-Mod Management

- List of active rules with enable/disable toggle
- Create new rule modal with form
- Rule types: Word Filter, Spam Detection, Raid Protection, Custom
- Each rule type has specific config fields
- Test rule button (optional: preview matches)

**Why:** Visual interface makes complex rule configuration accessible.

**How to apply:** Dynamic form based on rule_type, JSON schema validation, optimistic updates.

#### 6.5 Statistics Dashboard

- Time-series charts (bans, warns, kicks over time)
- Top moderators by action count
- Most warned users
- Rule effectiveness metrics
- Period selector (7d, 30d, 90d, all)

**Why:** Data visualization helps identify trends and measure moderation effectiveness.

**How to apply:** Recharts LineChart and BarChart components, React Query for data fetching.

#### 6.6 Settings Page

- Guild-specific configuration form
- Punishment escalation thresholds
- Default mute/ban durations
- Notification channels
- Moderator roles

**Why:** Centralized configuration reduces need for bot commands.

**How to apply:** React Hook Form with Zod validation, save button with loading state.

### UI/UX Principles

- **Responsive design** - mobile-friendly (Tailwind breakpoints)
- **Loading states** - skeletons for data fetching
- **Error handling** - toast notifications for errors
- **Confirmation dialogs** - for destructive actions (ban, delete rule)
- **Accessibility** - ARIA labels, keyboard navigation

---

## 7. Docker Setup

### docker-compose.yml Structure

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: discord_bot
      POSTGRES_USER: botuser
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U botuser"]
      interval: 10s
      timeout: 5s
      retries: 5

  bot:
    build: ./bot
    environment:
      DISCORD_TOKEN: ${DISCORD_TOKEN}
      DATABASE_URL: postgres://botuser:${DB_PASSWORD}@postgres:5432/discord_bot
      JWT_SECRET: ${JWT_SECRET}
      API_PORT: 8080
      FRONTEND_URL: http://localhost:3000
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    volumes:
      - ./logs:/app/logs

  frontend:
    build: ./frontend
    environment:
      NEXT_PUBLIC_API_URL: http://localhost:8080
    ports:
      - "3000:3000"
    depends_on:
      - bot
    restart: unless-stopped

volumes:
  postgres_data:
```

### Environment Variables

**.env file:**
```
DISCORD_TOKEN=your_discord_bot_token
DB_PASSWORD=secure_password_here
JWT_SECRET=random_secret_key
```

**Why:** Environment variables keep secrets out of code and enable different configs per environment.

**How to apply:** Use .env.example as template, add .env to .gitignore, validate required vars on startup.

### Dockerfile (Bot)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bot .
EXPOSE 8080
CMD ["./bot"]
```

### Dockerfile (Frontend)

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json
EXPOSE 3000
CMD ["npm", "start"]
```

### Deployment Steps

1. Clone repository
2. Copy `.env.example` to `.env` and fill values
3. Run `docker-compose up -d`
4. Run migrations: `docker-compose exec bot ./bot migrate`
5. Access admin panel at `http://localhost:3000`

**Why:** Docker ensures consistent environment across development and production.

**How to apply:** Multi-stage builds reduce image size, health checks ensure proper startup order.

---

## 8. Security

### Authentication & Authorization

- **Password hashing**: bcrypt with cost factor 12
- **JWT tokens**: 24-hour expiry, signed with HS256
- **Role-based access**: admin (full access), moderator (read-only logs)
- **Token refresh**: optional refresh token for extended sessions

**Why:** Layered security prevents unauthorized access and credential compromise.

**How to apply:** Middleware checks JWT on protected routes, bcrypt.CompareHashAndPassword for login.

### API Security

- **Rate limiting**: 100 requests/minute per IP (adjustable per endpoint)
- **CORS**: whitelist frontend URL only
- **Input validation**: validate all request bodies with struct tags
- **SQL injection**: use parameterized queries (pgx handles this)
- **XSS prevention**: sanitize user input before storing

**Why:** Prevents abuse, injection attacks, and unauthorized access.

**How to apply:** Use gin middleware for rate limiting and CORS, validator package for input validation.

### Discord Bot Security

- **Permission checks**: verify user has required Discord permissions before executing commands
- **Command cooldowns**: prevent spam (e.g., 5 seconds between commands per user)
- **Audit logging**: log all moderation actions with moderator ID
- **Graceful degradation**: if database unavailable, queue actions in memory

**Why:** Ensures only authorized users can moderate, prevents abuse.

**How to apply:** Check interaction.Member.Permissions before command execution, use sync.Map for cooldown tracking.

---

## 9. Error Handling & Logging

### Error Handling Strategy

**API Errors:**
- Centralized error handler middleware
- Consistent error response format: `{ error: string, code: string, details?: any }`
- HTTP status codes: 400 (validation), 401 (auth), 403 (forbidden), 404 (not found), 500 (server error)

**Bot Errors:**
- Retry logic for Discord API rate limits (exponential backoff)
- Fallback responses for command failures
- Database transaction rollback on errors

**Why:** Consistent error handling improves debugging and user experience.

**How to apply:** Custom error types in Go, middleware catches panics and returns JSON errors.

### Logging

**Structured Logging:**
- Format: JSON with fields (timestamp, level, message, context)
- Levels: DEBUG (development), INFO (startup, config changes), WARN (rate limits, retries), ERROR (failures)
- Destinations: stdout + file rotation (100MB max, 7 days retention)

**Log Categories:**
- `bot.commands` - command executions
- `bot.events` - Discord events
- `bot.automod` - auto-mod actions
- `api.requests` - HTTP requests
- `db.queries` - slow queries (>100ms)

**Why:** Structured logs enable filtering, searching, and monitoring.

**How to apply:** Use zap logger with JSON encoder, rotate logs with lumberjack.

### Health Checks

**Endpoints:**
- `GET /health` - basic liveness check
- `GET /health/ready` - readiness check (database connection, Discord connection)

**Monitoring:**
- Expose metrics endpoint (optional: Prometheus format)
- Track: command count, error rate, database query time, active guilds

**Why:** Health checks enable automated monitoring and alerting.

**How to apply:** Simple HTTP handlers that ping database and check Discord session state.

---

## 10. Testing Strategy

### Unit Tests (Go)

**Coverage targets:**
- Auto-mod engine logic (word filter, spam detection) - 90%+
- Punishment escalation logic - 90%+
- API handlers - 80%+
- Utility functions - 80%+

**Tools:**
- `testing` package (standard library)
- `testify/assert` for assertions
- `testify/mock` for mocking Discord API

**Why:** Unit tests catch logic errors early and enable safe refactoring.

**How to apply:** Test files alongside source (`handler_test.go`), run with `go test ./...`.

### Integration Tests (Go)

**Scenarios:**
- Database operations (CRUD for all tables)
- API endpoint flows (register → login → create rule)
- Discord event processing (mock events)

**Tools:**
- `testcontainers-go` for PostgreSQL test instance
- `httptest` for API testing

**Why:** Integration tests verify components work together correctly.

**How to apply:** Separate `integration_test.go` files, run with build tag `go test -tags=integration`.

### E2E Tests (Frontend)

**Critical flows:**
- Login → view guilds → view logs
- Create auto-mod rule → verify in list
- Ban user via API → verify in logs

**Tools:**
- Playwright or Cypress
- Mock API responses for isolated testing

**Why:** E2E tests ensure user-facing features work end-to-end.

**How to apply:** `tests/e2e/` directory, run in CI before deployment.

### Test Discord Server

- Dedicated server for development
- Test bot instance with separate token
- Automated test scenarios (send messages, trigger auto-mod)

**Why:** Real Discord environment catches API integration issues.

**How to apply:** Document test server setup in README, use separate .env for test bot.

---

## 11. CI/CD Pipeline

### GitHub Actions Workflow

**On Pull Request:**
1. Lint Go code (`golangci-lint`)
2. Lint TypeScript (`eslint`, `tsc`)
3. Run unit tests (Go)
4. Run integration tests (Go)
5. Build Docker images (verify no errors)

**On Merge to Main:**
1. Run all tests
2. Build and tag Docker images
3. Push to container registry (Docker Hub, GHCR)
4. Deploy to staging environment
5. Run smoke tests
6. (Manual approval) Deploy to production

**Why:** Automated pipeline catches issues before production and enables rapid iteration.

**How to apply:** `.github/workflows/ci.yml` and `.github/workflows/deploy.yml`.

### Deployment Strategy

**Staging:**
- Separate VPS or Docker Compose stack
- Uses test Discord bot token
- Deployed automatically on main branch changes

**Production:**
- Manual approval gate after staging validation
- Blue-green deployment (optional: run new version alongside old, switch traffic)
- Rollback plan: revert to previous Docker image tag

**Why:** Staging environment catches environment-specific issues, manual gate prevents accidental production changes.

**How to apply:** Separate `.env.staging` and `.env.production`, deployment scripts in `scripts/deploy.sh`.

---

## 12. Database Migrations

### Migration Strategy

**Tool:** golang-migrate or custom migration runner

**Migration files:**
```
migrations/
  000001_initial_schema.up.sql
  000001_initial_schema.down.sql
  000002_add_banned_words.up.sql
  000002_add_banned_words.down.sql
```

**Execution:**
- Run on bot startup (check version table)
- Or manual command: `./bot migrate`

**Why:** Version-controlled schema changes enable safe database evolution.

**How to apply:** Embed migrations in binary with `embed` package, run sequentially on startup.

### Backup Strategy

**Automated backups:**
- Daily PostgreSQL dump via cron job
- Retention: 7 daily, 4 weekly, 12 monthly
- Store in separate location (S3, external drive)

**Restore procedure:**
1. Stop bot container
2. Restore database from dump
3. Restart bot container

**Why:** Backups protect against data loss from bugs, corruption, or user error.

**How to apply:** `pg_dump` script in cron, document restore steps in README.

---

## 13. Documentation

### README.md

**Sections:**
- Project overview
- Features list
- Prerequisites (Docker, Discord bot token)
- Quick start guide
- Configuration options
- Development setup
- Deployment instructions
- Troubleshooting

### API Documentation

**Format:** OpenAPI 3.0 (Swagger)

**Generation:** Manual YAML file or code annotations (swaggo/swag)

**Hosting:** Serve at `/api/docs` via Swagger UI

**Why:** API docs enable frontend development without reading backend code.

**How to apply:** `docs/api.yaml`, mount Swagger UI in Docker container.

### Bot Commands Guide

**User-facing documentation:**
- List of all slash commands
- Required permissions
- Usage examples
- Auto-mod rule configuration guide

**Format:** Markdown in `docs/commands.md`

**Why:** Users need to understand how to use the bot.

**How to apply:** Generate from code comments or maintain manually.

### Development Guide

**Topics:**
- Project structure
- Adding new commands
- Adding new API endpoints
- Database schema changes
- Testing guidelines
- Code style conventions

**Format:** Markdown in `docs/development.md`

**Why:** Onboarding documentation helps contributors get started quickly.

**How to apply:** Document as you build, update when patterns change.

---

## 14. Future Enhancements (Out of Scope)

These features are not included in the initial implementation but can be added later:

- **Multi-language support** - i18n for bot responses and admin panel
- **Webhooks** - notify external services of moderation events
- **Advanced analytics** - ML-based toxicity detection, sentiment analysis
- **Plugin system** - custom modules for guild-specific features
- **Mobile app** - React Native admin panel
- **Voice moderation** - auto-disconnect, voice channel limits
- **Appeal system** - users can appeal bans/mutes through web form
- **Scheduled actions** - auto-unban after X days, scheduled announcements

**Why:** These add complexity and are not required for core functionality. Implement based on user feedback.

**How to apply:** Revisit after initial deployment and user testing.

---

## 15. Success Criteria

The system is considered complete when:

1. ✅ Bot connects to Discord and responds to slash commands
2. ✅ All moderation commands work (ban, kick, mute, warn)
3. ✅ Auto-moderation rules can be configured and execute correctly
4. ✅ Admin panel allows login and displays all guilds
5. ✅ Moderation logs are viewable and filterable
6. ✅ Statistics dashboard shows accurate data
7. ✅ Docker Compose deployment works on fresh VPS
8. ✅ All critical paths have test coverage
9. ✅ Documentation is complete and accurate
10. ✅ System handles 10-20 guilds with <100ms API response time

---

## 16. Project Structure

```
discordbot/
├── bot/                      # Go bot and API
│   ├── cmd/
│   │   └── bot/
│   │       └── main.go       # Entry point
│   ├── internal/
│   │   ├── api/              # REST API handlers
│   │   ├── bot/              # Discord bot logic
│   │   ├── automod/          # Auto-moderation engine
│   │   ├── database/         # Database layer
│   │   └── models/           # Data models
│   ├── migrations/           # SQL migrations
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── frontend/                 # Next.js admin panel
│   ├── app/                  # App router pages
│   ├── components/           # React components
│   ├── lib/                  # Utilities, API client
│   ├── public/               # Static assets
│   ├── Dockerfile
│   ├── package.json
│   └── tsconfig.json
├── docs/                     # Documentation
│   ├── api.yaml              # OpenAPI spec
│   ├── commands.md           # Bot commands guide
│   └── development.md        # Dev guide
├── docker-compose.yml
├── .env.example
└── README.md
```

---

## Conclusion

This design provides a complete, production-ready Discord moderation system with:
- Comprehensive moderation features (manual + automated)
- User-friendly admin panel for configuration and monitoring
- Scalable architecture for 10-20 servers
- Security best practices
- Docker-based deployment
- Testing and CI/CD pipeline

The monolithic approach balances simplicity with functionality, allowing rapid development while maintaining a clear migration path to microservices if scale demands it.
