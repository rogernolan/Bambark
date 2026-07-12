package notification

import (
	"fmt"
	"strings"
)

type BambuddyPayload struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Test    string `json:"test"`
	Push    string `json:"push"`
}

type Notification struct {
	Title string
	Body  string
}

func FromBambuddy(payload BambuddyPayload) (Notification, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = strings.TrimSpace(payload.Test)
	}
	if title == "" {
		return Notification{}, fmt.Errorf("missing title")
	}

	body := strings.TrimSpace(payload.Message)
	if body == "" {
		body = strings.TrimSpace(payload.Push)
	}
	if body == "" {
		return Notification{}, fmt.Errorf("missing message")
	}

	return Notification{Title: title, Body: body}, nil
}
