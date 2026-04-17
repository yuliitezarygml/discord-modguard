package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/discordbot/bot/internal/models"
	"go.uber.org/zap"
)

var slashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "ban",
		Description: "Ban a user from the server",
		DefaultMemberPermissions: permPtr(discordgo.PermissionBanMembers),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to ban", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
			{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration (e.g. 1h, 30m, 7d)", Required: false},
		},
	},
	{
		Name:        "kick",
		Description: "Kick a user from the server",
		DefaultMemberPermissions: permPtr(discordgo.PermissionKickMembers),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to kick", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
		},
	},
	{
		Name:        "mute",
		Description: "Mute (timeout) a user",
		DefaultMemberPermissions: permPtr(discordgo.PermissionModerateMembers),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to mute", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "Duration (e.g. 1h, 30m)", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
		},
	},
	{
		Name:        "unmute",
		Description: "Remove timeout from a user",
		DefaultMemberPermissions: permPtr(discordgo.PermissionModerateMembers),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to unmute", Required: true},
		},
	},
	{
		Name:        "warn",
		Description: "Issue a warning to a user",
		DefaultMemberPermissions: permPtr(discordgo.PermissionModerateMembers),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: true},
		},
	},
	{
		Name:        "warnings",
		Description: "List warnings for a user",
		DefaultMemberPermissions: permPtr(discordgo.PermissionModerateMembers),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: true},
		},
	},
	{
		Name:        "clearwarnings",
		Description: "Clear active warnings for a user",
		DefaultMemberPermissions: permPtr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: true},
		},
	},
}

func permPtr(p int64) *int64 { return &p }

func (b *Bot) registerCommands() error {
	_, err := b.Session.ApplicationCommandBulkOverwrite(b.AppID, "", slashCommands)
	return err
}

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	ctx := context.Background()
	data := i.ApplicationCommandData()
	switch data.Name {
	case "ban":
		b.cmdBan(ctx, s, i)
	case "kick":
		b.cmdKick(ctx, s, i)
	case "mute":
		b.cmdMute(ctx, s, i)
	case "unmute":
		b.cmdUnmute(ctx, s, i)
	case "warn":
		b.cmdWarn(ctx, s, i)
	case "warnings":
		b.cmdWarnings(ctx, s, i)
	case "clearwarnings":
		b.cmdClearWarnings(ctx, s, i)
	}
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	data := &discordgo.InteractionResponseData{Content: content}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

func optMap(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(opts))
	for _, o := range opts {
		m[o.Name] = o
	}
	return m
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func (b *Bot) logAction(ctx context.Context, i *discordgo.InteractionCreate, action, targetID, targetName string, reason *string, dur *int64) {
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	var modID int64
	var modName string
	if i.Member != nil && i.Member.User != nil {
		modID, _ = strconv.ParseInt(i.Member.User.ID, 10, 64)
		modName = i.Member.User.Username
	}
	tid, _ := strconv.ParseInt(targetID, 10, 64)
	l := &models.ModerationLog{
		GuildID:        gid,
		ModeratorID:    modID,
		ModeratorName:  modName,
		ActionType:     action,
		TargetUserID:   tid,
		TargetUsername: targetName,
		Reason:         reason,
		DurationSec:    dur,
	}
	if err := b.Store.InsertLog(ctx, l); err != nil {
		b.Log.Error("insert log", zap.Error(err))
	}
}

func (b *Bot) cmdBan(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	reason := ""
	if o, ok := opts["reason"]; ok {
		reason = o.StringValue()
	}
	var durSec *int64
	var expiresAt *time.Time
	if o, ok := opts["duration"]; ok {
		d, err := parseDuration(o.StringValue())
		if err != nil {
			respond(s, i, "Invalid duration: "+err.Error(), true)
			return
		}
		if d > 0 {
			sec := int64(d.Seconds())
			durSec = &sec
			t := time.Now().Add(d)
			expiresAt = &t
		}
	}

	if err := s.GuildBanCreateWithReason(i.GuildID, user.ID, reason, 0); err != nil {
		respond(s, i, "Failed to ban: "+err.Error(), true)
		return
	}

	var r *string
	if reason != "" {
		r = &reason
	}
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(user.ID, 10, 64)
	_ = b.Store.AddPunishment(ctx, &models.Punishment{
		GuildID: gid, UserID: uid, Type: "ban", Reason: r,
		DurationSec: durSec, ExpiresAt: expiresAt, Active: true,
	})
	b.logAction(ctx, i, "ban", user.ID, user.Username, r, durSec)

	msg := fmt.Sprintf("Banned %s", user.Username)
	if durSec != nil {
		msg += fmt.Sprintf(" for %s", formatDuration(*durSec))
	}
	if reason != "" {
		msg += ". Reason: " + reason
	}
	respond(s, i, msg, false)
}

func (b *Bot) cmdKick(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	reason := ""
	if o, ok := opts["reason"]; ok {
		reason = o.StringValue()
	}
	if err := s.GuildMemberDeleteWithReason(i.GuildID, user.ID, reason); err != nil {
		respond(s, i, "Failed to kick: "+err.Error(), true)
		return
	}
	var r *string
	if reason != "" {
		r = &reason
	}
	b.logAction(ctx, i, "kick", user.ID, user.Username, r, nil)
	msg := "Kicked " + user.Username
	if reason != "" {
		msg += ". Reason: " + reason
	}
	respond(s, i, msg, false)
}

func (b *Bot) cmdMute(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	d, err := parseDuration(opts["duration"].StringValue())
	if err != nil || d <= 0 {
		respond(s, i, "Invalid duration", true)
		return
	}
	reason := ""
	if o, ok := opts["reason"]; ok {
		reason = o.StringValue()
	}
	until := time.Now().Add(d)
	if err := s.GuildMemberTimeout(i.GuildID, user.ID, &until); err != nil {
		respond(s, i, "Failed to mute: "+err.Error(), true)
		return
	}
	sec := int64(d.Seconds())
	var r *string
	if reason != "" {
		r = &reason
	}
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(user.ID, 10, 64)
	_ = b.Store.AddPunishment(ctx, &models.Punishment{
		GuildID: gid, UserID: uid, Type: "mute", Reason: r,
		DurationSec: &sec, ExpiresAt: &until, Active: true,
	})
	b.logAction(ctx, i, "mute", user.ID, user.Username, r, &sec)
	msg := fmt.Sprintf("Muted %s for %s", user.Username, formatDuration(sec))
	if reason != "" {
		msg += ". Reason: " + reason
	}
	respond(s, i, msg, false)
}

func (b *Bot) cmdUnmute(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	if err := s.GuildMemberTimeout(i.GuildID, user.ID, nil); err != nil {
		respond(s, i, "Failed to unmute: "+err.Error(), true)
		return
	}
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(user.ID, 10, 64)
	_ = b.Store.DeactivatePunishments(ctx, gid, uid, "mute")
	b.logAction(ctx, i, "unmute", user.ID, user.Username, nil, nil)
	respond(s, i, "Unmuted "+user.Username, false)
}

func (b *Bot) cmdWarn(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	reason := opts["reason"].StringValue()
	var modID int64
	var modName string
	if i.Member != nil && i.Member.User != nil {
		modID, _ = strconv.ParseInt(i.Member.User.ID, 10, 64)
		modName = i.Member.User.Username
	}
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(user.ID, 10, 64)
	w := &models.Warning{
		GuildID:       gid,
		UserID:        uid,
		Username:      user.Username,
		Reason:        &reason,
		ModeratorID:   modID,
		ModeratorName: modName,
	}
	if err := b.Store.AddWarning(ctx, w); err != nil {
		respond(s, i, "Failed to store warning: "+err.Error(), true)
		return
	}
	b.logAction(ctx, i, "warn", user.ID, user.Username, &reason, nil)

	warns, _ := b.Store.UserWarnings(ctx, gid, uid, true)
	count := len(warns)
	msg := fmt.Sprintf("Warned %s (%d active warnings). Reason: %s", user.Username, count, reason)
	b.maybeEscalate(ctx, s, i, user, count)
	respond(s, i, msg, false)
}

func (b *Bot) maybeEscalate(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, user *discordgo.User, count int) {
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	g, _ := b.Store.GetGuild(ctx, gid)
	muteAt, banAt := 3, 5
	if g != nil && g.SettingsJSON != nil {
		if v, ok := g.SettingsJSON["warn_mute_threshold"].(float64); ok {
			muteAt = int(v)
		}
		if v, ok := g.SettingsJSON["warn_ban_threshold"].(float64); ok {
			banAt = int(v)
		}
	}
	if count >= banAt {
		_ = s.GuildBanCreateWithReason(i.GuildID, user.ID, "warning threshold reached", 0)
		uid, _ := strconv.ParseInt(user.ID, 10, 64)
		reason := "warning threshold reached"
		_ = b.Store.AddPunishment(ctx, &models.Punishment{
			GuildID: gid, UserID: uid, Type: "ban", Reason: &reason, Active: true,
		})
		b.logAction(ctx, i, "ban", user.ID, user.Username, &reason, nil)
	} else if count >= muteAt {
		until := time.Now().Add(1 * time.Hour)
		_ = s.GuildMemberTimeout(i.GuildID, user.ID, &until)
		uid, _ := strconv.ParseInt(user.ID, 10, 64)
		reason := "warning threshold reached"
		sec := int64(3600)
		_ = b.Store.AddPunishment(ctx, &models.Punishment{
			GuildID: gid, UserID: uid, Type: "mute", Reason: &reason,
			DurationSec: &sec, ExpiresAt: &until, Active: true,
		})
		b.logAction(ctx, i, "mute", user.ID, user.Username, &reason, &sec)
	}
}

func (b *Bot) cmdWarnings(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(user.ID, 10, 64)
	ws, err := b.Store.UserWarnings(ctx, gid, uid, false)
	if err != nil {
		respond(s, i, "Query failed", true)
		return
	}
	if len(ws) == 0 {
		respond(s, i, user.Username+" has no warnings", true)
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Warnings for %s:\n", user.Username)
	for _, w := range ws {
		reason := ""
		if w.Reason != nil {
			reason = *w.Reason
		}
		status := "active"
		if !w.Active {
			status = "cleared"
		}
		fmt.Fprintf(&sb, "- [%s] %s by %s (%s)\n",
			w.CreatedAt.Format("2006-01-02"), reason, w.ModeratorName, status)
	}
	respond(s, i, sb.String(), true)
}

func (b *Bot) cmdClearWarnings(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	opts := optMap(i.ApplicationCommandData().Options)
	user := opts["user"].UserValue(s)
	gid, _ := strconv.ParseInt(i.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(user.ID, 10, 64)
	n, err := b.Store.ClearWarnings(ctx, gid, uid)
	if err != nil {
		respond(s, i, "Failed: "+err.Error(), true)
		return
	}
	respond(s, i, fmt.Sprintf("Cleared %d warnings for %s", n, user.Username), false)
}

func formatDuration(sec int64) string {
	d := time.Duration(sec) * time.Second
	return d.String()
}
