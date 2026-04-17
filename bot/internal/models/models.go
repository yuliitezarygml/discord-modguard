package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Guild struct {
	ID           int64          `json:"id,string"`
	Name         string         `json:"name"`
	SettingsJSON map[string]any `json:"settings"`
	AddedAt      time.Time      `json:"added_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	MemberCount  int            `json:"member_count,omitempty"`
	IconURL      string         `json:"icon_url,omitempty"`
	BannerURL    string         `json:"banner_url,omitempty"`
}

type ModerationLog struct {
	ID             uuid.UUID `json:"id"`
	GuildID        int64     `json:"guild_id,string"`
	ModeratorID    int64     `json:"moderator_id,string"`
	ModeratorName  string    `json:"moderator_name"`
	ActionType     string    `json:"action_type"`
	TargetUserID   int64     `json:"target_user_id,string"`
	TargetUsername string    `json:"target_username"`
	Reason         *string   `json:"reason"`
	DurationSec    *int64    `json:"duration_seconds"`
	CreatedAt      time.Time `json:"created_at"`
}

type AutoModRule struct {
	ID         uuid.UUID      `json:"id"`
	GuildID    int64          `json:"guild_id,string"`
	RuleType   string         `json:"rule_type"`
	ConfigJSON map[string]any `json:"config"`
	Enabled    bool           `json:"enabled"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type Warning struct {
	ID            uuid.UUID `json:"id"`
	GuildID       int64     `json:"guild_id,string"`
	UserID        int64     `json:"user_id,string"`
	Username      string    `json:"username"`
	Reason        *string   `json:"reason"`
	ModeratorID   int64     `json:"moderator_id,string"`
	ModeratorName string    `json:"moderator_name"`
	CreatedAt     time.Time `json:"created_at"`
	Active        bool      `json:"active"`
}

type BannedWord struct {
	ID        uuid.UUID `json:"id"`
	GuildID   int64     `json:"guild_id,string"`
	Word      string    `json:"word"`
	IsRegex   bool      `json:"is_regex"`
	CreatedAt time.Time `json:"created_at"`
}

type Punishment struct {
	ID          uuid.UUID  `json:"id"`
	GuildID     int64      `json:"guild_id,string"`
	UserID      int64      `json:"user_id,string"`
	Type        string     `json:"type"`
	Reason      *string    `json:"reason"`
	DurationSec *int64     `json:"duration_seconds"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Active      bool       `json:"active"`
}
