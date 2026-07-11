package bark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rog/bambark/internal/notification"
)

type Client struct {
	baseURL    *url.URL
	deviceKey  string
	httpClient *http.Client
}

type barkRequest struct {
	Body      string `json:"body"`
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
}

type ResponseError struct {
	Code   int
	Status string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("bark request failed: %s", e.Status)
}

func (e *ResponseError) StatusCode() int {
	return e.Code
}

func (e *ResponseError) StatusText() string {
	return e.Status
}

func NewClient(baseURL, deviceKey string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    parsed,
		deviceKey:  deviceKey,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Send(ctx context.Context, notification notification.Notification) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/push"

	payload := barkRequest{
		Body:      notification.Body,
		DeviceKey: c.deviceKey,
		Title:     notification.Title,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		return &ResponseError{Code: resp.StatusCode, Status: resp.Status}
	}

	return nil
}
