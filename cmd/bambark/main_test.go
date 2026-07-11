package main

import (
	"net/http"
	"testing"

	"github.com/rog/bambark/internal/config"
)

func TestNewServerBuildsConfiguredHTTPServer(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		ListenAddr:         "127.0.0.1:8080",
		BarkURL:            "http://127.0.0.1:8081",
		BarkDeviceKey:      "device-key",
		WebhookBearerToken: "secret",
	}

	server, err := newServer(cfg, &http.Client{})
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	if server.Addr != cfg.ListenAddr {
		t.Fatalf("server.Addr = %q, want %q", server.Addr, cfg.ListenAddr)
	}
	if server.Handler == nil {
		t.Fatal("server.Handler = nil, want non-nil handler")
	}
}

func TestNewServerRejectsInvalidBarkURL(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		ListenAddr:         "127.0.0.1:8080",
		BarkURL:            "://bad-url",
		BarkDeviceKey:      "device-key",
		WebhookBearerToken: "secret",
	}

	server, err := newServer(cfg, &http.Client{})
	if err == nil {
		t.Fatal("newServer() error = nil, want invalid Bark URL error")
	}
	if server != nil {
		t.Fatalf("newServer() server = %#v, want nil on error", server)
	}
}
