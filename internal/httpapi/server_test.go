package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rog/bambark/internal/notification"
)

func TestHandlerAcceptsAuthenticatedBambuddyWebhook(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"}`))
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if sender.got != (notification.Notification{Title: "Print done", Body: "Finished"}) {
		t.Fatalf("notification = %#v", sender.got)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}
}

func TestHandlerRejectsMissingOrWrongBearerToken(t *testing.T) {
	for _, tc := range []struct {
		name          string
		authorization string
	}{
		{name: "missing"},
		{name: "wrong", authorization: "Bearer nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &recordingSender{}
			handler := NewServer("secret", sender, time.Second).Handler()
			r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"}`))
			if tc.authorization != "" {
				r.Header.Set("Authorization", tc.authorization)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
			if sender.calls != 0 {
				t.Fatalf("sender calls = %d, want 0", sender.calls)
			}
		})
	}
}

func TestHandlerRejectsMalformedJSON(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestHandlerRejectsTrailingJSONValues(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"}{"extra":true}`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestHandlerRejectsBlankRequiredFields(t *testing.T) {
	for _, body := range []string{
		`{"title":"","message":"Finished"}`,
		`{"title":"Print done","message":"   "}`,
		`{"message":"Finished"}`,
		`{"title":"Print done"}`,
	} {
		t.Run(body, func(t *testing.T) {
			sender := &recordingSender{}
			handler := NewServer("secret", sender, time.Second).Handler()
			r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer secret")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
			if sender.calls != 0 {
				t.Fatalf("sender calls = %d, want 0", sender.calls)
			}
		})
	}
}

func TestHandlerRejectsOverlyLargeRequestBody(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"`+strings.Repeat("A", 1<<20)+`","message":"Finished"}`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestHandlerReturnsBadGatewayWhenSenderFails(t *testing.T) {
	sender := &recordingSender{err: errors.New("boom")}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"}`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}
}

func TestHandlerUsesTimeoutContextForSender(t *testing.T) {
	sender := &recordingSender{}
	timeout := 250 * time.Millisecond
	handler := NewServer("secret", sender, timeout).Handler()
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"}`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if !sender.hadDeadline {
		t.Fatal("sender context had no deadline")
	}
	remaining := sender.deadline.Sub(start)
	if remaining <= 0 || remaining > timeout+200*time.Millisecond {
		t.Fatalf("deadline remaining = %v, want within (0, %v]", remaining, timeout+200*time.Millisecond)
	}
}

func TestHandlerRejectsUnsupportedMethod(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodGet, "/webhook/bambuddy", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestHealthzReturnsOKWithoutCallingSender(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestHealthzRejectsUnsupportedMethod(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

type recordingSender struct {
	got         notification.Notification
	err         error
	calls       int
	hadDeadline bool
	deadline    time.Time
}

func (s *recordingSender) Send(ctx context.Context, got notification.Notification) error {
	s.calls++
	s.got = got
	s.deadline, s.hadDeadline = ctx.Deadline()
	return s.err
}
