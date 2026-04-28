package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const defaultHTTPAddr = ":8080"
const defaultWebDistDir = "web/dist"
const DefaultAppURL = "http://localhost:8080"
const DefaultDatabaseURL = "sqlite://data/arqboard.db"
const defaultSessionSecret = "local-development-session-secret-change-me"

type Config struct {
	AppEnv        string
	DatabaseURL   string
	AppURL        string
	SessionSecret string
	HTTPAddr      string
	WebDistDir    string
}

type LookupFunc func(string) (string, bool)

func FromEnv() (Config, error) {
	return Load(os.LookupEnv)
}

func Load(lookup LookupFunc) (Config, error) {
	cfg := Config{
		AppEnv:        env(lookup, "APP_ENV"),
		DatabaseURL:   env(lookup, "DATABASE_URL"),
		AppURL:        env(lookup, "APP_URL"),
		SessionSecret: env(lookup, "SESSION_SECRET"),
		HTTPAddr:      env(lookup, "HTTP_ADDR"),
		WebDistDir:    env(lookup, "WEB_DIST_DIR"),
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.WebDistDir == "" {
		cfg.WebDistDir = defaultWebDistDir
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}
	if cfg.AppEnv != "production" {
		if cfg.DatabaseURL == "" {
			cfg.DatabaseURL = DefaultDatabaseURL
		}
		if cfg.AppURL == "" {
			cfg.AppURL = DefaultAppURL
		}
		if cfg.SessionSecret == "" {
			cfg.SessionSecret = defaultSessionSecret
		}
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.AppURL == "" {
		missing = append(missing, "APP_URL")
	}
	if cfg.SessionSecret == "" {
		missing = append(missing, "SESSION_SECRET")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if len(cfg.SessionSecret) < 32 {
		return Config{}, errors.New("SESSION_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func env(lookup LookupFunc, key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
