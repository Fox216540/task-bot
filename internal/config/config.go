package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	BotToken      string
	DBPath        string
	AllowedUserID int64
}

func Load() (Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("BOT_TOKEN is required")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "task_bot.sqlite"
	}

	allowedUserIDRaw := os.Getenv("ALLOWED_USER_ID")
	if allowedUserIDRaw == "" {
		return Config{}, fmt.Errorf("ALLOWED_USER_ID is required")
	}
	allowedUserID, err := strconv.ParseInt(allowedUserIDRaw, 10, 64)
	if err != nil || allowedUserID <= 0 {
		return Config{}, fmt.Errorf("ALLOWED_USER_ID must be a positive integer")
	}

	return Config{
		BotToken:      token,
		DBPath:        dbPath,
		AllowedUserID: allowedUserID,
	}, nil
}
