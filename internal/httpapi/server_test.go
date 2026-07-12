package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rog/bambark/internal/bark"
	"github.com/rog/bambark/internal/notification"
)

func TestHandlerAcceptsAuthenticatedBambuddyWebhook(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := newBambuddyRequest(`{"title":"Print done","message":"Finished"}`)
	r.Header.Set("Authorization", "Bearer secret")
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

func TestHandlerLogsAcceptedWebhookWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	sender := &recordingSender{}
	handler := NewServer("super-secret-token", sender, time.Second, log.New(&logs, "", 0)).Handler()
	r := newBambuddyRequest(`{"title":"Printer jammed","message":"Door open"}`)
	r.Header.Set("Authorization", "Bearer super-secret-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	output := logs.String()
	for _, want := range []string{
		"endpoint=/webhook/bambuddy",
		"status=Accepted",
		"outcome=accepted",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("log output %q does not contain %q", output, want)
		}
	}
	for _, leak := range []string{
		"super-secret-token",
		"Printer jammed",
		"Door open",
	} {
		if strings.Contains(output, leak) {
			t.Fatalf("log output %q leaked %q", output, leak)
		}
	}
}

func TestHandlerAcceptsJSONWithParametersAndExtraFields(t *testing.T) {
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second).Handler()
	r := newBambuddyRequest(`{"title":"Print done","message":"Finished","event":"print_done","ignored":true}`)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
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

func TestHandlerLogsRejectedWebhookWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	sender := &recordingSender{}
	handler := NewServer("secret", sender, time.Second, log.New(&logs, "", 0)).Handler()
	r := newBambuddyRequest(`{"title":"Printer jammed","message":"Door open"}`)
	r.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	output := logs.String()
	for _, want := range []string{
		"endpoint=/webhook/bambuddy",
		"status=Unauthorized",
		"outcome=rejected",
		"reason=unauthorized",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("log output %q does not contain %q", output, want)
		}
	}
	for _, leak := range []string{
		"wrong-token",
		"Printer jammed",
		"Door open",
	} {
		if strings.Contains(output, leak) {
			t.Fatalf("log output %q leaked %q", output, leak)
		}
	}
}

func TestHandlerRejectsMissingOrWrongContentType(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
	}{
		{name: "missing"},
		{name: "wrong", contentType: "text/plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &recordingSender{}
			handler := NewServer("secret", sender, time.Second).Handler()
			r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(`{"title":"Print done","message":"Finished"}`))
			r.Header.Set("Authorization", "Bearer secret")
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
			}
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

func TestHandlerRejectsUnauthorizedRequestsBeforeContentTypeValidation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
	}{
		{name: "missing"},
		{name: "wrong", contentType: "text/plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &recordingSender{}
			handler := NewServer("secret", sender, time.Second).Handler()
			r := newBambuddyRequest(`{"title":"Print done","message":"Finished"}`)
			r.Header.Set("Authorization", "Bearer wrong-token")
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
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

func TestHandlerLogsBarkFailureStatusWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	sender := &recordingSender{err: statusReportingError{code: http.StatusBadRequest, text: http.StatusText(http.StatusBadRequest)}}
	handler := NewServer("secret", sender, time.Second, log.New(&logs, "", 0)).Handler()
	r := newBambuddyRequest(`{"title":"Printer jammed","message":"Door open"}`)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	output := logs.String()
	for _, want := range []string{
		"endpoint=/webhook/bambuddy",
		"status=Bad Gateway",
		"outcome=rejected",
		"reason=bark_status_failure",
		"bark_status_code=400",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("log output %q does not contain %q", output, want)
		}
	}
	for _, leak := range []string{
		"secret",
		"Printer jammed",
		"Door open",
	} {
		if strings.Contains(output, leak) {
			t.Fatalf("log output %q leaked %q", output, leak)
		}
	}
}

func TestHandlerLogsRealBarkFailureStatusWithoutDuplicateCodes(t *testing.T) {
	var logs bytes.Buffer
	barkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer barkServer.Close()

	sender, err := bark.NewClient(barkServer.URL, "device-key", barkServer.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	handler := NewServer("secret", sender, time.Second, log.New(&logs, "", 0)).Handler()
	r := newBambuddyRequest(`{"title":"Printer jammed","message":"Door open"}`)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	output := logs.String()
	for _, want := range []string{
		"endpoint=/webhook/bambuddy",
		"status=Bad Gateway",
		"outcome=rejected",
		"reason=bark_status_failure",
		"bark_status_code=400",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("log output %q does not contain %q", output, want)
		}
	}
	if got := strings.Count(output, "400"); got != 1 {
		t.Fatalf("log output %q contains %d occurrences of 400, want 1", output, got)
	}
	for _, leak := range []string{
		"secret",
		"Printer jammed",
		"Door open",
	} {
		if strings.Contains(output, leak) {
			t.Fatalf("log output %q leaked %q", output, leak)
		}
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
			r := newBambuddyRequest(`{"title":"Print done","message":"Finished"}`)
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
	r := newBambuddyRequest(`{"title":"Print done","message":"Finished"`)
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
	r := newBambuddyRequest(`{"title":"Print done","message":"Finished"}{"extra":true}`)
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
			r := newBambuddyRequest(body)
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
	r := newBambuddyRequest(`{"title":"` + strings.Repeat("A", 1<<20) + `","message":"Finished"}`)
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
	r := newBambuddyRequest(`{"title":"Print done","message":"Finished"}`)
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
	r := newBambuddyRequest(`{"title":"Print done","message":"Finished"}`)
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

type statusReportingError struct {
	code int
	text string
}

func (e statusReportingError) Error() string {
	return "bark request failed"
}

func (e statusReportingError) StatusCode() int {
	return e.code
}

func (e statusReportingError) StatusText() string {
	return e.text
}

func (s *recordingSender) Send(ctx context.Context, got notification.Notification) error {
	s.calls++
	s.got = got
	s.deadline, s.hadDeadline = ctx.Deadline()
	return s.err
}

func newBambuddyRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/webhook/bambuddy", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}
