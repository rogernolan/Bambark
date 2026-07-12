package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/rog/bambark/internal/notification"
)

const maxWebhookBodyBytes = 1 << 20

type Sender interface {
	Send(ctx context.Context, notification notification.Notification) error
}

type Logger interface {
	Printf(format string, v ...any)
}

type Server struct {
	expectedToken  string
	sender         Sender
	requestTimeout time.Duration
	logger         Logger
}

func NewServer(expectedToken string, sender Sender, requestTimeout time.Duration, logger ...Logger) *Server {
	var resolved Logger = noopLogger{}
	if len(logger) > 0 && logger[0] != nil {
		resolved = logger[0]
	}

	return &Server{
		expectedToken:  expectedToken,
		sender:         sender,
		requestTimeout: requestTimeout,
		logger:         resolved,
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		s.handleHealthz(w, r)
	case "/webhook/bambuddy":
		s.handleBambuddyWebhook(w, r)
	default:
		s.logWebhookOutcome(r.URL.Path, http.StatusNotFound, "rejected", "reason=not_found")
		http.NotFound(w, r)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleBambuddyWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.logWebhookOutcome(r.URL.Path, http.StatusMethodNotAllowed, "rejected", "reason=method_not_allowed")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	token, ok := BearerToken(r)
	if !ok || token != s.expectedToken {
		s.logWebhookOutcome(r.URL.Path, http.StatusUnauthorized, "rejected", "reason=unauthorized")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodGet {
		payload := notification.BambuddyPayload{
			Title:   firstQueryValue(r, "title", "subject"),
			Message: firstQueryValue(r, "message", "body", "text"),
		}
		got, err := notification.FromBambuddy(payload)
		if err != nil {
			s.logWebhookOutcome(r.URL.Path, http.StatusBadRequest, "rejected", "reason=invalid_payload", "received="+quote(r.URL.RawQuery))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		s.deliverNotification(w, r, got)
		return
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		s.logWebhookOutcome(r.URL.Path, http.StatusBadRequest, "rejected", "reason=invalid_content_type")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	payload, received, ok := decodeBambuddyPayload(w, r)
	if !ok {
		s.logWebhookOutcome(r.URL.Path, http.StatusBadRequest, "rejected", "reason=invalid_json")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	got, err := notification.FromBambuddy(payload)
	if err != nil {
		s.logWebhookOutcome(r.URL.Path, http.StatusBadRequest, "rejected", "reason=invalid_payload", "received="+quote(received))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	s.deliverNotification(w, r, got)
}

func (s *Server) deliverNotification(w http.ResponseWriter, r *http.Request, got notification.Notification) {
	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	if err := s.sender.Send(ctx, got); err != nil {
		if code, ok := statusInfo(err); ok {
			s.logWebhookOutcome(
				r.URL.Path,
				http.StatusBadGateway,
				"rejected",
				"reason=bark_status_failure",
				"bark_status_code="+fmt.Sprintf("%d", code),
			)
		} else {
			s.logWebhookOutcome(r.URL.Path, http.StatusBadGateway, "rejected", "reason=bark_failure", "error="+quote(err.Error()))
		}
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	s.logWebhookOutcome(r.URL.Path, http.StatusAccepted, "accepted")
	w.WriteHeader(http.StatusAccepted)
}

func firstQueryValue(r *http.Request, keys ...string) string {
	query := r.URL.Query()
	for _, key := range keys {
		if value := query.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) logWebhookOutcome(endpoint string, status int, outcome string, details ...string) {
	fields := []string{
		"endpoint=" + endpoint,
		"status=" + http.StatusText(status),
		"outcome=" + outcome,
	}
	fields = append(fields, details...)
	s.logger.Printf("webhook %s", strings.Join(fields, " "))
}

type noopLogger struct{}

func (noopLogger) Printf(string, ...any) {}

type barkStatusProvider interface {
	StatusCode() int
}

func statusInfo(err error) (int, bool) {
	provider, ok := err.(barkStatusProvider)
	if !ok {
		return 0, false
	}

	return provider.StatusCode(), true
}

func quote(value string) string {
	return fmt.Sprintf("%q", value)
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "application/json")
}

func decodeBambuddyPayload(w http.ResponseWriter, r *http.Request) (notification.BambuddyPayload, string, bool) {
	body := http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	defer body.Close()

	raw, err := io.ReadAll(body)
	if err != nil {
		return notification.BambuddyPayload{}, "", false
	}

	var payload notification.BambuddyPayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&payload); err != nil {
		return notification.BambuddyPayload{}, "", false
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return notification.BambuddyPayload{}, "", false
	}

	return payload, string(raw), true
}
