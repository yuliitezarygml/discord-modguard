package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/discordbot/bot/internal/models"
	"github.com/discordbot/bot/internal/store"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	Store     *store.Store
	Discord   *discordgo.Session
	Log       *zap.Logger
	JWTSecret string
	Frontend  string
	Port      string
}

func (s *Server) Router() http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.logMiddleware())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{s.Frontend},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/health/ready", s.ready)

	api := r.Group("/api")
	auth := api.Group("/auth")
	auth.POST("/register", s.register)
	auth.POST("/login", s.login)
	auth.POST("/logout", s.logout)

	protected := api.Group("")
	protected.Use(s.authMiddleware())
	protected.GET("/me", s.me)
	protected.GET("/guilds", s.listGuilds)
	protected.GET("/guilds/:id", s.getGuild)
	protected.PUT("/guilds/:id/settings", s.adminOnly(), s.updateGuildSettings)
	protected.GET("/guilds/:id/logs", s.listLogs)
	protected.GET("/guilds/:id/warnings", s.listWarnings)
	protected.POST("/guilds/:id/ban", s.adminOnly(), s.banUser)
	protected.DELETE("/guilds/:id/ban/:userId", s.adminOnly(), s.unbanUser)
	protected.GET("/guilds/:id/automod", s.listRules)
	protected.POST("/guilds/:id/automod", s.adminOnly(), s.createRule)
	protected.PUT("/guilds/:id/automod/:ruleId", s.adminOnly(), s.updateRule)
	protected.DELETE("/guilds/:id/automod/:ruleId", s.adminOnly(), s.deleteRule)
	protected.GET("/guilds/:id/stats", s.guildStats)

	return r
}

func (s *Server) logMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.Log.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("dur", time.Since(start)),
		)
	}
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing token"})
			return
		}
		claims, err := parseToken(strings.TrimPrefix(h, "Bearer "), s.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func (s *Server) adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "admin only"})
			return
		}
		c.Next()
	}
}

func (s *Server) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c, 2*time.Second)
	defer cancel()
	if _, err := s.Store.ListGuilds(ctx); err != nil {
		c.JSON(503, gin.H{"ok": false, "db": false})
		return
	}
	c.JSON(200, gin.H{"ok": true, "db": true, "discord": s.Discord != nil})
}

// ---- Auth ----

type registerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (s *Server) register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "validation", "details": err.Error()})
		return
	}
	role := "admin"
	n, err := s.Store.CountUsers(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	if n > 0 {
		role = "moderator"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		c.JSON(500, gin.H{"error": "hash"})
		return
	}
	u, err := s.Store.CreateUser(c, strings.ToLower(req.Email), string(hash), role)
	if err != nil {
		c.JSON(400, gin.H{"error": "user exists or invalid"})
		return
	}
	tok, _ := issueToken(s.JWTSecret, u.ID, u.Role, 24*time.Hour)
	c.JSON(200, gin.H{"user": u, "token": tok})
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "validation"})
		return
	}
	u, err := s.Store.UserByEmail(c, strings.ToLower(req.Email))
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	tok, _ := issueToken(s.JWTSecret, u.ID, u.Role, 24*time.Hour)
	c.JSON(200, gin.H{"user": u, "token": tok})
}

func (s *Server) logout(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *Server) me(c *gin.Context) {
	id, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(401, gin.H{"error": "bad token"})
		return
	}
	u, err := s.Store.UserByID(c, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, u)
}

// ---- Guilds ----

func (s *Server) listGuilds(c *gin.Context) {
	gs, err := s.Store.ListGuilds(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	for i := range gs {
		s.enrichGuild(&gs[i])
	}
	c.JSON(200, gs)
}

func (s *Server) enrichGuild(g *models.Guild) {
	if s.Discord == nil {
		return
	}
	dg, err := s.Discord.State.Guild(strconv.FormatInt(g.ID, 10))
	if err != nil {
		return
	}
	g.MemberCount = dg.MemberCount
	if dg.Icon != "" {
		ext := "png"
		if strings.HasPrefix(dg.Icon, "a_") {
			ext = "gif"
		}
		g.IconURL = fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.%s?size=256", dg.ID, dg.Icon, ext)
	}
	if dg.Banner != "" {
		ext := "png"
		if strings.HasPrefix(dg.Banner, "a_") {
			ext = "gif"
		}
		g.BannerURL = fmt.Sprintf("https://cdn.discordapp.com/banners/%s/%s.%s?size=1024", dg.ID, dg.Banner, ext)
	}
}

func (s *Server) getGuild(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	g, err := s.Store.GetGuild(c, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	s.enrichGuild(g)
	c.JSON(200, g)
}

type settingsReq struct {
	Settings map[string]any `json:"settings"`
}

func (s *Server) updateGuildSettings(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	var req settingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "validation"})
		return
	}
	if err := s.Store.UpdateGuildSettings(c, id, req.Settings); err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	g, _ := s.Store.GetGuild(c, id)
	c.JSON(200, g)
}

// ---- Logs ----

func (s *Server) listLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 200 {
		limit = 200
	}
	if page < 1 {
		page = 1
	}
	modID, _ := strconv.ParseInt(c.Query("moderator_id"), 10, 64)
	targetID, _ := strconv.ParseInt(c.Query("target_user_id"), 10, 64)
	logs, total, err := s.Store.ListLogs(c, store.LogsFilter{
		GuildID:     id,
		ActionType:  c.Query("action_type"),
		ModeratorID: modID,
		TargetID:    targetID,
		Limit:       limit,
		Offset:      (page - 1) * limit,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	if logs == nil {
		logs = []models.ModerationLog{}
	}
	c.JSON(200, gin.H{"logs": logs, "total": total, "page": page, "limit": limit})
}

func (s *Server) listWarnings(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	uid, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if uid == 0 {
		c.JSON(400, gin.H{"error": "user_id required"})
		return
	}
	ws, err := s.Store.UserWarnings(c, id, uid, false)
	if err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	if ws == nil {
		ws = []models.Warning{}
	}
	c.JSON(200, ws)
}

type banReq struct {
	UserID   string `json:"user_id" binding:"required"`
	Reason   string `json:"reason"`
	Duration string `json:"duration"`
}

func (s *Server) banUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	var req banReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "validation"})
		return
	}
	if s.Discord == nil {
		c.JSON(503, gin.H{"error": "discord not ready"})
		return
	}
	gidStr := strconv.FormatInt(id, 10)
	if err := s.Discord.GuildBanCreateWithReason(gidStr, req.UserID, req.Reason, 0); err != nil {
		c.JSON(500, gin.H{"error": "discord", "details": err.Error()})
		return
	}
	uid, _ := strconv.ParseInt(req.UserID, 10, 64)
	var durSec *int64
	var expires *time.Time
	if req.Duration != "" {
		if d, err := parseDur(req.Duration); err == nil {
			sec := int64(d.Seconds())
			durSec = &sec
			t := time.Now().Add(d)
			expires = &t
		}
	}
	var r *string
	if req.Reason != "" {
		r = &req.Reason
	}
	_ = s.Store.AddPunishment(c, &models.Punishment{
		GuildID: id, UserID: uid, Type: "ban", Reason: r,
		DurationSec: durSec, ExpiresAt: expires, Active: true,
	})
	log := &models.ModerationLog{
		GuildID:      id,
		ModeratorID:  panelModID(c),
		ModeratorName: "panel",
		ActionType:   "ban",
		TargetUserID: uid,
		Reason:       r,
		DurationSec:  durSec,
	}
	_ = s.Store.InsertLog(c, log)
	c.JSON(200, gin.H{"success": true, "log": log})
}

func (s *Server) unbanUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	userID := c.Param("userId")
	if s.Discord == nil {
		c.JSON(503, gin.H{"error": "discord not ready"})
		return
	}
	gidStr := strconv.FormatInt(id, 10)
	if err := s.Discord.GuildBanDelete(gidStr, userID); err != nil {
		c.JSON(500, gin.H{"error": "discord", "details": err.Error()})
		return
	}
	uid, _ := strconv.ParseInt(userID, 10, 64)
	_ = s.Store.DeactivatePunishments(c, id, uid, "ban")
	_ = s.Store.InsertLog(c, &models.ModerationLog{
		GuildID:       id,
		ModeratorID:   panelModID(c),
		ModeratorName: "panel",
		ActionType:    "unban",
		TargetUserID:  uid,
	})
	c.JSON(200, gin.H{"success": true})
}

// ---- Automod ----

func (s *Server) listRules(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	rules, err := s.Store.ListRules(c, id)
	if err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	if rules == nil {
		rules = []models.AutoModRule{}
	}
	c.JSON(200, rules)
}

type ruleReq struct {
	RuleType string         `json:"rule_type" binding:"required"`
	Config   map[string]any `json:"config"`
	Enabled  *bool          `json:"enabled"`
}

func (s *Server) createRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "validation"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	r := &models.AutoModRule{
		GuildID:    id,
		RuleType:   req.RuleType,
		ConfigJSON: req.Config,
		Enabled:    enabled,
	}
	if err := s.Store.CreateRule(c, r); err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	c.JSON(200, r)
}

type ruleUpdateReq struct {
	Config  map[string]any `json:"config"`
	Enabled *bool          `json:"enabled"`
}

func (s *Server) updateRule(c *gin.Context) {
	rid, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "bad rule id"})
		return
	}
	var req ruleUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "validation"})
		return
	}
	r, err := s.Store.UpdateRule(c, rid, req.Config, req.Enabled)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	c.JSON(200, r)
}

func (s *Server) deleteRule(c *gin.Context) {
	rid, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "bad rule id"})
		return
	}
	if err := s.Store.DeleteRule(c, rid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// ---- Stats ----

func (s *Server) guildStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "bad id"})
		return
	}
	period := c.DefaultQuery("period", "30d")
	since := time.Now().AddDate(-10, 0, 0)
	switch period {
	case "7d":
		since = time.Now().AddDate(0, 0, -7)
	case "30d":
		since = time.Now().AddDate(0, 0, -30)
	case "90d":
		since = time.Now().AddDate(0, 0, -90)
	}
	st, err := s.Store.Stats(c, id, since)
	if err != nil {
		c.JSON(500, gin.H{"error": "db"})
		return
	}
	c.JSON(200, st)
}

// ---- helpers ----

func panelModID(c *gin.Context) int64 {
	return 0
}

func parseDur(s string) (time.Duration, error) {
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
