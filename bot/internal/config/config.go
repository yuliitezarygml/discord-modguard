package config

import (
	"fmt"
	"os"
)

type Config struct {
	DiscordToken string
	DiscordAppID string
	DatabaseURL  string
	JWTSecret    string
	APIPort      string
	FrontendURL  string
}

func Load() (*Config, error) {
	c := &Config{
		DiscordToken: os.Getenv("DISCORD_TOKEN"),
		DiscordAppID: os.Getenv("DISCORD_APP_ID"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		APIPort:      getEnv("API_PORT", "8080"),
		FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:3000"),
	}
	if c.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN is required")
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
