package httpapi

import (
	"net/http"
	"strings"
)

func BearerToken(r *http.Request) (string, bool) {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 {
		return "", false
	}

	if !strings.EqualFold(fields[0], "Bearer") {
		return "", false
	}

	if strings.TrimSpace(fields[1]) == "" {
		return "", false
	}

	return fields[1], true
}
