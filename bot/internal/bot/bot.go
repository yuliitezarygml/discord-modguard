package bot

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/discordbot/bot/internal/automod"
	"github.com/discordbot/bot/internal/store"
	"go.uber.org/zap"
)

type Bot struct {
	Session *discordgo.Session
	Store   *store.Store
	Log     *zap.Logger
	Automod *automod.Engine
	AppID   string
}

func New(token, appID string, st *store.Store, log *zap.Logger) (*Bot, error) {
	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord new: %w", err)
	}
	sess.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildBans

	b := &Bot{
		Session: sess,
		Store:   st,
		Log:     log,
		Automod: automod.New(st, log),
		AppID:   appID,
	}
	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start(ctx context.Context) error {
	if err := b.Session.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}
	if b.AppID == "" {
		b.AppID = b.Session.State.User.ID
	}
	if err := b.registerCommands(); err != nil {
		b.Log.Error("register commands", zap.Error(err))
	}
	b.syncGuilds(ctx)
	go b.punishmentTicker(ctx)
	return nil
}

func (b *Bot) Stop() error {
	return b.Session.Close()
}

func (b *Bot) syncGuilds(ctx context.Context) {
	for _, g := range b.Session.State.Guilds {
		gid, _ := strconv.ParseInt(g.ID, 10, 64)
		if err := b.Store.UpsertGuild(ctx, gid, g.Name); err != nil {
			b.Log.Error("upsert guild", zap.Error(err), zap.String("guild_id", g.ID))
		}
	}
}

func (b *Bot) punishmentTicker(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			b.expirePunishments(ctx, now)
		}
	}
}

func (b *Bot) expirePunishments(ctx context.Context, now time.Time) {
	ps, err := b.Store.ExpiredPunishments(ctx, now)
	if err != nil {
		b.Log.Error("expired punishments query", zap.Error(err))
		return
	}
	for _, p := range ps {
		gid := strconv.FormatInt(p.GuildID, 10)
		uid := strconv.FormatInt(p.UserID, 10)
		switch p.Type {
		case "ban":
			if err := b.Session.GuildBanDelete(gid, uid); err != nil {
				b.Log.Warn("auto-unban failed", zap.Error(err))
			}
		case "mute", "timeout":
			if err := b.Session.GuildMemberTimeout(gid, uid, nil); err != nil {
				b.Log.Warn("auto-unmute failed", zap.Error(err))
			}
		}
		_ = b.Store.DeactivatePunishments(ctx, p.GuildID, p.UserID, p.Type)
	}
}
