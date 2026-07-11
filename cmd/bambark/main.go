package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rog/bambark/internal/bark"
	"github.com/rog/bambark/internal/config"
	"github.com/rog/bambark/internal/httpapi"
)

const (
	barkClientTimeout       = 10 * time.Second
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 10 * time.Second
	serverIdleTimeout       = 60 * time.Second
	shutdownTimeout         = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Printf("bambark stopped with error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	server, err := newServer(cfg, &http.Client{Timeout: barkClientTimeout})
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("bambark listening on %s", server.Addr)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		log.Printf("bambark shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	return <-serverErrors
}

func newServer(cfg config.Config, client *http.Client) (*http.Server, error) {
	barkClient, err := bark.NewClient(cfg.BarkURL, cfg.BarkDeviceKey, client)
	if err != nil {
		return nil, err
	}

	handler := httpapi.NewServer(cfg.WebhookBearerToken, barkClient, barkClientTimeout).Handler()

	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}, nil
}
