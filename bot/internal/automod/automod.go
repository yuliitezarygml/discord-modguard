package automod

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/discordbot/bot/internal/models"
	"github.com/discordbot/bot/internal/store"
	"go.uber.org/zap"
)

type Engine struct {
	store *store.Store
	log   *zap.Logger

	mu         sync.Mutex
	msgHistory map[int64]map[int64][]time.Time // guildID -> userID -> timestamps
	joins      map[int64][]time.Time           // guildID -> recent joins
}

func New(st *store.Store, log *zap.Logger) *Engine {
	return &Engine{
		store:      st,
		log:        log,
		msgHistory: map[int64]map[int64][]time.Time{},
		joins:      map[int64][]time.Time{},
	}
}

func (e *Engine) Handle(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	gid, _ := strconv.ParseInt(m.GuildID, 10, 64)
	rules, err := e.store.ActiveRules(ctx, gid)
	if err != nil {
		return
	}

	for _, r := range rules {
		switch r.RuleType {
		case "word_filter":
			if e.checkWordFilter(r, m.Content) {
				e.act(ctx, s, m, r, "word filter matched")
				return
			}
		case "spam_detection":
			if e.checkSpam(r, gid, m) {
				e.act(ctx, s, m, r, "spam detected")
				return
			}
		}
	}
}

func (e *Engine) checkWordFilter(r models.AutoModRule, content string) bool {
	words, _ := r.ConfigJSON["words"].([]any)
	lc := strings.ToLower(content)
	for _, w := range words {
		s, ok := w.(string)
		if !ok || s == "" {
			continue
		}
		if strings.Contains(lc, strings.ToLower(s)) {
			return true
		}
	}
	patterns, _ := r.ConfigJSON["patterns"].([]any)
	for _, p := range patterns {
		s, ok := p.(string)
		if !ok || s == "" {
			continue
		}
		re, err := regexp.Compile(s)
		if err == nil && re.MatchString(content) {
			return true
		}
	}
	return false
}

func (e *Engine) checkSpam(r models.AutoModRule, gid int64, m *discordgo.MessageCreate) bool {
	limit := 5
	windowSec := 10
	if v, ok := r.ConfigJSON["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := r.ConfigJSON["window_seconds"].(float64); ok {
		windowSec = int(v)
	}
	uid, _ := strconv.ParseInt(m.Author.ID, 10, 64)
	now := time.Now()
	cutoff := now.Add(-time.Duration(windowSec) * time.Second)

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.msgHistory[gid] == nil {
		e.msgHistory[gid] = map[int64][]time.Time{}
	}
	hist := e.msgHistory[gid][uid]
	fresh := hist[:0]
	for _, t := range hist {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	fresh = append(fresh, now)
	e.msgHistory[gid][uid] = fresh
	return len(fresh) > limit
}

func (e *Engine) act(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, r models.AutoModRule, reason string) {
	action, _ := r.ConfigJSON["action"].(string)
	if action == "" {
		action = "delete"
	}
	gid, _ := strconv.ParseInt(m.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(m.Author.ID, 10, 64)

	switch action {
	case "delete":
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
	case "warn":
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		_ = e.store.AddWarning(ctx, &models.Warning{
			GuildID: gid, UserID: uid, Username: m.Author.Username,
			Reason: &reason, ModeratorName: "automod",
		})
	case "mute":
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		until := time.Now().Add(10 * time.Minute)
		_ = s.GuildMemberTimeout(m.GuildID, m.Author.ID, &until)
		sec := int64(600)
		_ = e.store.AddPunishment(ctx, &models.Punishment{
			GuildID: gid, UserID: uid, Type: "mute",
			Reason: &reason, DurationSec: &sec, ExpiresAt: &until, Active: true,
		})
	case "ban":
		_ = s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, reason, 0)
		_ = e.store.AddPunishment(ctx, &models.Punishment{
			GuildID: gid, UserID: uid, Type: "ban", Reason: &reason, Active: true,
		})
	}

	_ = e.store.InsertLog(ctx, &models.ModerationLog{
		GuildID:        gid,
		ModeratorID:    0,
		ModeratorName:  "automod",
		ActionType:     "automod",
		TargetUserID:   uid,
		TargetUsername: m.Author.Username,
		Reason:         &reason,
	})
}

func (e *Engine) RaidCheck(ctx context.Context, s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	gid, _ := strconv.ParseInt(m.GuildID, 10, 64)
	rules, err := e.store.ActiveRules(ctx, gid)
	if err != nil {
		return
	}
	var rule *models.AutoModRule
	for i := range rules {
		if rules[i].RuleType == "raid_protection" {
			rule = &rules[i]
			break
		}
	}
	if rule == nil {
		return
	}
	limit := 10
	windowSec := 30
	if v, ok := rule.ConfigJSON["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := rule.ConfigJSON["window_seconds"].(float64); ok {
		windowSec = int(v)
	}
	alertChan, _ := rule.ConfigJSON["alert_channel_id"].(string)

	now := time.Now()
	cutoff := now.Add(-time.Duration(windowSec) * time.Second)
	e.mu.Lock()
	fresh := e.joins[gid][:0]
	for _, t := range e.joins[gid] {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	fresh = append(fresh, now)
	e.joins[gid] = fresh
	count := len(fresh)
	e.mu.Unlock()

	if count > limit && alertChan != "" {
		_, _ = s.ChannelMessageSend(alertChan,
			"⚠️ Raid protection: "+strconv.Itoa(count)+" joins in last "+
				strconv.Itoa(windowSec)+"s")
	}
}
