package bot

import (
	"context"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/discordbot/bot/internal/models"
	"go.uber.org/zap"
)

func (b *Bot) registerHandlers() {
	b.Session.AddHandler(b.onReady)
	b.Session.AddHandler(b.onInteraction)
	b.Session.AddHandler(b.onMessageCreate)
	b.Session.AddHandler(b.onGuildCreate)
	b.Session.AddHandler(b.onGuildDelete)
	b.Session.AddHandler(b.onGuildMemberAdd)
	b.Session.AddHandler(b.onGuildBanAdd)
	b.Session.AddHandler(b.onGuildBanRemove)
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	b.Log.Info("discord ready", zap.String("user", r.User.Username), zap.Int("guilds", len(r.Guilds)))
}

func (b *Bot) onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	gid, _ := strconv.ParseInt(g.ID, 10, 64)
	if err := b.Store.UpsertGuild(context.Background(), gid, g.Name); err != nil {
		b.Log.Error("upsert guild", zap.Error(err))
	}
}

func (b *Bot) onGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	if g.Unavailable {
		return
	}
	gid, _ := strconv.ParseInt(g.ID, 10, 64)
	_ = b.Store.DeleteGuild(context.Background(), gid)
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.GuildID == "" {
		return
	}
	ctx := context.Background()
	b.Automod.Handle(ctx, s, m)
}

func (b *Bot) onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	b.Automod.RaidCheck(context.Background(), s, m)
}

func (b *Bot) onGuildBanAdd(s *discordgo.Session, e *discordgo.GuildBanAdd) {
	gid, _ := strconv.ParseInt(e.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(e.User.ID, 10, 64)
	_ = b.Store.InsertLog(context.Background(), &models.ModerationLog{
		GuildID:        gid,
		ModeratorID:    0,
		ActionType:     "ban",
		TargetUserID:   uid,
		TargetUsername: e.User.Username,
	})
}

func (b *Bot) onGuildBanRemove(s *discordgo.Session, e *discordgo.GuildBanRemove) {
	gid, _ := strconv.ParseInt(e.GuildID, 10, 64)
	uid, _ := strconv.ParseInt(e.User.ID, 10, 64)
	_ = b.Store.DeactivatePunishments(context.Background(), gid, uid, "ban")
	_ = b.Store.InsertLog(context.Background(), &models.ModerationLog{
		GuildID:        gid,
		ActionType:     "unban",
		TargetUserID:   uid,
		TargetUsername: e.User.Username,
	})
}
