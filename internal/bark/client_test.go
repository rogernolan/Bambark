package bark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rog/bambark/internal/notification"
)

func TestClientSendsBarkPushJSON(t *testing.T) {
	var got barkRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/push" {
			t.Fatalf("request = %s %s, want POST /push", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "device-key", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Send(context.Background(), notification.Notification{Title: "Title", Body: "Body"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got.DeviceKey != "device-key" || got.Title != "Title" || got.Body != "Body" {
		t.Fatalf("Bark payload = %#v", got)
	}
}

func TestClientReturnsErrorForBarkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "device-key", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Send(context.Background(), notification.Notification{Title: "Title", Body: "Body"}); err == nil {
		t.Fatal("Send() error = nil, want Bark failure")
	}
}

func TestClientReturnsErrorForTransportFailure(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1", "device-key", &http.Client{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Send(context.Background(), notification.Notification{Title: "Title", Body: "Body"}); err == nil {
		t.Fatal("Send() error = nil, want transport failure")
	}
}
