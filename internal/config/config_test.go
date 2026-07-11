package config

import (
	"strings"
	"testing"
)

func TestLoadUsesDefaultListenAddress(t *testing.T) {
	getenv := func(key string) string {
		return map[string]string{
			"BARK_URL":             "http://bark:8080",
			"BARK_DEVICE_KEY":      "device-key",
			"WEBHOOK_BEARER_TOKEN": "secret",
		}[key]
	}

	got, err := Load(getenv)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q, want %q", got.ListenAddr, ":8080")
	}
}

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("Load() error = nil, want missing configuration error")
	}
	for _, name := range []string{"BARK_URL", "BARK_DEVICE_KEY", "WEBHOOK_BEARER_TOKEN"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error %q does not mention %s", err, name)
		}
	}
}
