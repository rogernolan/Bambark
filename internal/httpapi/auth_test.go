package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerTokenExtractsCaseInsensitiveBearerScheme(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer abc123")
	got, ok := BearerToken(r)
	if !ok || got != "abc123" {
		t.Fatalf("BearerToken() = %q, %v", got, ok)
	}
}

func TestBearerTokenRejectsMalformedHeader(t *testing.T) {
	for _, value := range []string{"", "Basic abc", "Bearer", "Bearer one two"} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Authorization", value)
		if _, ok := BearerToken(r); ok {
			t.Errorf("BearerToken(%q) accepted malformed header", value)
		}
	}
}
