package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerTokenExtractsCaseInsensitiveBearerScheme(t *testing.T) {
	for _, value := range []string{"Bearer abc123", "bEaReR abc123"} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Authorization", value)
		got, ok := BearerToken(r)
		if !ok || got != "abc123" {
			t.Fatalf("BearerToken(%q) = %q, %v", value, got, ok)
		}
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
