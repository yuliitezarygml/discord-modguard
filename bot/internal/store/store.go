package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/discordbot/bot/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ---- Users ----

func (s *Store) CreateUser(ctx context.Context, email, hash, role string) (*models.User, error) {
	u := &models.User{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1,$2,$3)
		 RETURNING id, email, password_hash, role, created_at, updated_at`,
		email, hash, role,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE email=$1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u := &models.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE id=$1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// ---- Guilds ----

func (s *Store) UpsertGuild(ctx context.Context, id int64, name string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO guilds (id, name) VALUES ($1,$2)
		 ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, updated_at=NOW()`,
		id, name,
	)
	return err
}

func (s *Store) DeleteGuild(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM guilds WHERE id=$1`, id)
	return err
}

func (s *Store) ListGuilds(ctx context.Context) ([]models.Guild, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, settings_json, added_at, updated_at FROM guilds ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Guild
	for rows.Next() {
		var g models.Guild
		var settings []byte
		if err := rows.Scan(&g.ID, &g.Name, &settings, &g.AddedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if len(settings) > 0 {
			_ = json.Unmarshal(settings, &g.SettingsJSON)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) GetGuild(ctx context.Context, id int64) (*models.Guild, error) {
	g := &models.Guild{}
	var settings []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, settings_json, added_at, updated_at FROM guilds WHERE id=$1`, id,
	).Scan(&g.ID, &g.Name, &settings, &g.AddedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &g.SettingsJSON)
	}
	return g, nil
}

func (s *Store) UpdateGuildSettings(ctx context.Context, id int64, settings map[string]any) error {
	b, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE guilds SET settings_json=$2, updated_at=NOW() WHERE id=$1`, id, b)
	return err
}

// ---- Moderation Logs ----

func (s *Store) InsertLog(ctx context.Context, l *models.ModerationLog) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO moderation_logs
		 (guild_id, moderator_id, moderator_name, action_type, target_user_id, target_username, reason, duration_seconds)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, created_at`,
		l.GuildID, l.ModeratorID, l.ModeratorName, l.ActionType,
		l.TargetUserID, l.TargetUsername, l.Reason, l.DurationSec,
	).Scan(&l.ID, &l.CreatedAt)
}

type LogsFilter struct {
	GuildID     int64
	ActionType  string
	ModeratorID int64
	TargetID    int64
	Limit       int
	Offset      int
}

func (s *Store) ListLogs(ctx context.Context, f LogsFilter) ([]models.ModerationLog, int, error) {
	q := `SELECT id, guild_id, moderator_id, moderator_name, action_type, target_user_id,
			target_username, reason, duration_seconds, created_at
			FROM moderation_logs WHERE guild_id=$1`
	args := []any{f.GuildID}
	i := 2
	if f.ActionType != "" {
		q += " AND action_type=$" + itoa(i)
		args = append(args, f.ActionType)
		i++
	}
	if f.ModeratorID != 0 {
		q += " AND moderator_id=$" + itoa(i)
		args = append(args, f.ModeratorID)
		i++
	}
	if f.TargetID != 0 {
		q += " AND target_user_id=$" + itoa(i)
		args = append(args, f.TargetID)
		i++
	}

	countQ := "SELECT COUNT(*) FROM (" + q + ") s"
	var total int
	if err := s.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q += " ORDER BY created_at DESC LIMIT $" + itoa(i) + " OFFSET $" + itoa(i+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.ModerationLog
	for rows.Next() {
		var l models.ModerationLog
		if err := rows.Scan(&l.ID, &l.GuildID, &l.ModeratorID, &l.ModeratorName,
			&l.ActionType, &l.TargetUserID, &l.TargetUsername, &l.Reason,
			&l.DurationSec, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// ---- Warnings ----

func (s *Store) AddWarning(ctx context.Context, w *models.Warning) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO warnings (guild_id, user_id, username, reason, moderator_id, moderator_name)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, active`,
		w.GuildID, w.UserID, w.Username, w.Reason, w.ModeratorID, w.ModeratorName,
	).Scan(&w.ID, &w.CreatedAt, &w.Active)
}

func (s *Store) UserWarnings(ctx context.Context, guildID, userID int64, activeOnly bool) ([]models.Warning, error) {
	q := `SELECT id, guild_id, user_id, username, reason, moderator_id, moderator_name, created_at, active
		  FROM warnings WHERE guild_id=$1 AND user_id=$2`
	if activeOnly {
		q += " AND active=TRUE"
	}
	q += " ORDER BY created_at DESC"
	rows, err := s.pool.Query(ctx, q, guildID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Warning
	for rows.Next() {
		var w models.Warning
		if err := rows.Scan(&w.ID, &w.GuildID, &w.UserID, &w.Username, &w.Reason,
			&w.ModeratorID, &w.ModeratorName, &w.CreatedAt, &w.Active); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) ClearWarnings(ctx context.Context, guildID, userID int64) (int64, error) {
	ct, err := s.pool.Exec(ctx,
		`UPDATE warnings SET active=FALSE WHERE guild_id=$1 AND user_id=$2 AND active=TRUE`,
		guildID, userID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// ---- Auto-mod rules ----

func (s *Store) ListRules(ctx context.Context, guildID int64) ([]models.AutoModRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, guild_id, rule_type, config_json, enabled, created_at, updated_at
		 FROM auto_mod_rules WHERE guild_id=$1 ORDER BY created_at DESC`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AutoModRule
	for rows.Next() {
		var r models.AutoModRule
		var cfg []byte
		if err := rows.Scan(&r.ID, &r.GuildID, &r.RuleType, &cfg, &r.Enabled,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if len(cfg) > 0 {
			_ = json.Unmarshal(cfg, &r.ConfigJSON)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ActiveRules(ctx context.Context, guildID int64) ([]models.AutoModRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, guild_id, rule_type, config_json, enabled, created_at, updated_at
		 FROM auto_mod_rules WHERE guild_id=$1 AND enabled=TRUE`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AutoModRule
	for rows.Next() {
		var r models.AutoModRule
		var cfg []byte
		if err := rows.Scan(&r.ID, &r.GuildID, &r.RuleType, &cfg, &r.Enabled,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if len(cfg) > 0 {
			_ = json.Unmarshal(cfg, &r.ConfigJSON)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateRule(ctx context.Context, r *models.AutoModRule) error {
	cfg, err := json.Marshal(r.ConfigJSON)
	if err != nil {
		return err
	}
	return s.pool.QueryRow(ctx,
		`INSERT INTO auto_mod_rules (guild_id, rule_type, config_json, enabled)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
		r.GuildID, r.RuleType, cfg, r.Enabled,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

func (s *Store) UpdateRule(ctx context.Context, id uuid.UUID, config map[string]any, enabled *bool) (*models.AutoModRule, error) {
	r := &models.AutoModRule{}
	var cfg []byte
	if config != nil {
		b, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		cfg = b
	}
	err := s.pool.QueryRow(ctx,
		`UPDATE auto_mod_rules SET
			config_json = COALESCE($2, config_json),
			enabled = COALESCE($3, enabled),
			updated_at = NOW()
		 WHERE id=$1
		 RETURNING id, guild_id, rule_type, config_json, enabled, created_at, updated_at`,
		id, cfg, enabled,
	).Scan(&r.ID, &r.GuildID, &r.RuleType, &cfg, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &r.ConfigJSON)
	}
	return r, nil
}

func (s *Store) DeleteRule(ctx context.Context, id uuid.UUID) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM auto_mod_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Punishments ----

func (s *Store) AddPunishment(ctx context.Context, p *models.Punishment) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO punishments (guild_id, user_id, type, reason, duration_seconds, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, active`,
		p.GuildID, p.UserID, p.Type, p.Reason, p.DurationSec, p.ExpiresAt,
	).Scan(&p.ID, &p.CreatedAt, &p.Active)
}

func (s *Store) DeactivatePunishments(ctx context.Context, guildID, userID int64, ptype string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE punishments SET active=FALSE
		 WHERE guild_id=$1 AND user_id=$2 AND type=$3 AND active=TRUE`,
		guildID, userID, ptype)
	return err
}

func (s *Store) ExpiredPunishments(ctx context.Context, now time.Time) ([]models.Punishment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, guild_id, user_id, type, reason, duration_seconds, expires_at, created_at, active
		 FROM punishments WHERE active=TRUE AND expires_at IS NOT NULL AND expires_at <= $1`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Punishment
	for rows.Next() {
		var p models.Punishment
		if err := rows.Scan(&p.ID, &p.GuildID, &p.UserID, &p.Type, &p.Reason,
			&p.DurationSec, &p.ExpiresAt, &p.CreatedAt, &p.Active); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---- Banned words ----

func (s *Store) BannedWords(ctx context.Context, guildID int64) ([]models.BannedWord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, guild_id, word, is_regex, created_at FROM banned_words WHERE guild_id=$1`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.BannedWord
	for rows.Next() {
		var w models.BannedWord
		if err := rows.Scan(&w.ID, &w.GuildID, &w.Word, &w.IsRegex, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---- Stats ----

type GuildStats struct {
	TotalBans   int              `json:"total_bans"`
	TotalKicks  int              `json:"total_kicks"`
	TotalWarns  int              `json:"total_warns"`
	TotalMutes  int              `json:"total_mutes"`
	ActiveRules int              `json:"active_rules"`
	Timeline    []TimelineBucket `json:"timeline"`
}

type TimelineBucket struct {
	Date  string `json:"date"`
	Bans  int    `json:"bans"`
	Kicks int    `json:"kicks"`
	Warns int    `json:"warns"`
	Mutes int    `json:"mutes"`
}

func (s *Store) Stats(ctx context.Context, guildID int64, since time.Time) (*GuildStats, error) {
	st := &GuildStats{}
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE action_type='ban') AS bans,
			COUNT(*) FILTER (WHERE action_type='kick') AS kicks,
			COUNT(*) FILTER (WHERE action_type='warn') AS warns,
			COUNT(*) FILTER (WHERE action_type='mute') AS mutes
		FROM moderation_logs WHERE guild_id=$1 AND created_at >= $2`,
		guildID, since,
	).Scan(&st.TotalBans, &st.TotalKicks, &st.TotalWarns, &st.TotalMutes)
	if err != nil {
		return nil, err
	}

	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auto_mod_rules WHERE guild_id=$1 AND enabled=TRUE`, guildID,
	).Scan(&st.ActiveRules)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS d,
			COUNT(*) FILTER (WHERE action_type='ban'),
			COUNT(*) FILTER (WHERE action_type='kick'),
			COUNT(*) FILTER (WHERE action_type='warn'),
			COUNT(*) FILTER (WHERE action_type='mute')
		FROM moderation_logs WHERE guild_id=$1 AND created_at >= $2
		GROUP BY d ORDER BY d ASC`, guildID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b TimelineBucket
		if err := rows.Scan(&b.Date, &b.Bans, &b.Kicks, &b.Warns, &b.Mutes); err != nil {
			return nil, err
		}
		st.Timeline = append(st.Timeline, b)
	}
	return st, rows.Err()
}

// helper
func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
