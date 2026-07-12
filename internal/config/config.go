package config

import (
	"fmt"
	"strings"
)

type Config struct {
	ListenAddr        string
	BarkURL           string
	BarkDeviceKey     string
	WebhookBearerToken string
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		ListenAddr:        getenv("LISTEN_ADDR"),
		BarkURL:           strings.TrimSpace(getenv("BARK_URL")),
		BarkDeviceKey:     strings.TrimSpace(getenv("BARK_DEVICE_KEY")),
		WebhookBearerToken: strings.TrimSpace(getenv("WEBHOOK_BEARER_TOKEN")),
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	var missing []string
	if cfg.BarkURL == "" {
		missing = append(missing, "BARK_URL")
	}
	if cfg.BarkDeviceKey == "" {
		missing = append(missing, "BARK_DEVICE_KEY")
	}
	if cfg.WebhookBearerToken == "" {
		missing = append(missing, "WEBHOOK_BEARER_TOKEN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
