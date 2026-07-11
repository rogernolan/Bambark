package httpapi

import (
	"context"
	"encoding/json"
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

type Server struct {
	expectedToken  string
	sender         Sender
	requestTimeout time.Duration
}

func NewServer(expectedToken string, sender Sender, requestTimeout time.Duration) *Server {
	return &Server{
		expectedToken:  expectedToken,
		sender:         sender,
		requestTimeout: requestTimeout,
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
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token, ok := BearerToken(r)
	if !ok || token != s.expectedToken {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	payload, ok := decodeBambuddyPayload(w, r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	got, err := notification.FromBambuddy(payload)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	if err := s.sender.Send(ctx, got); err != nil {
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "application/json")
}

func decodeBambuddyPayload(w http.ResponseWriter, r *http.Request) (notification.BambuddyPayload, bool) {
	body := http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	defer body.Close()

	var payload notification.BambuddyPayload
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&payload); err != nil {
		return notification.BambuddyPayload{}, false
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return notification.BambuddyPayload{}, false
	}

	return payload, true
}
