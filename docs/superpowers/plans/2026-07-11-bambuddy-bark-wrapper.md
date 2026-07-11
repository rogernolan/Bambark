# Bambuddy-to-Bark Webhook Wrapper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a small Go service that authenticates Bambuddy webhooks, maps them to Bark push requests, and runs persistently under systemd.

**Architecture:** The HTTP handler authenticates and decodes Bambuddy payloads into a source-neutral `Notification`. A Bark client sends that notification to the configured Bark `/push` endpoint. Configuration is validated at startup, while systemd and documentation provide long-lived Debian LXC operation.

**Tech Stack:** Go standard library, `net/http`, `encoding/json`, `httptest`, systemd.

## Global Constraints

- Support only `POST /webhook/bambuddy` as a notification input.
- Use one `BARK_URL`, one `BARK_DEVICE_KEY`, and one `WEBHOOK_BEARER_TOKEN` from environment variables.
- Return `202` only after Bark accepts the notification.
- Return `400` for malformed or incomplete payloads, `401` for bearer failures, and `502` for Bark failures.
- Keep `GET /healthz` unauthenticated and independent of Bark availability.
- Do not add a database, persistent queue, retry worker, multi-user behavior, or alternate delivery target.
- Use Go's standard library only for production dependencies.
- Write each production behavior test first, run it failing, then implement the minimum code to pass.

---

### Task 1: Create the Go module and configuration loader

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces `config.Config` with `ListenAddr`, `BarkURL`, `BarkDeviceKey`, and `WebhookBearerToken` string fields.
- Produces `config.Load(getenv func(string) string) (Config, error)`.

- [ ] **Step 1: Write the failing configuration tests**

```go
func TestLoadUsesDefaultListenAddress(t *testing.T) {
	getenv := func(key string) string {
		return map[string]string{
			"BARK_URL":             "http://bark:8080",
			"BARK_DEVICE_KEY":      "device-key",
			"WEBHOOK_BEARER_TOKEN": "secret",
		}[key]
	}

	got, err := Load(getenv)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q, want %q", got.ListenAddr, ":8080")
	}
}

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("Load() error = nil, want missing configuration error")
	}
	for _, name := range []string{"BARK_URL", "BARK_DEVICE_KEY", "WEBHOOK_BEARER_TOKEN"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error %q does not mention %s", err, name)
		}
	}
}
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/config`

Expected: FAIL because the module and `Load` function do not exist.

- [ ] **Step 3: Add the minimal module and loader**

Create `go.mod` with module path `github.com/rog/bambark` and `go 1.24`. Implement `Config` and `Load`; use `:8080` only when `LISTEN_ADDR` is empty, and return one error listing every missing required variable.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `go test ./internal/config`

Expected: PASS.

- [ ] **Step 5: Commit the configuration unit**

Run: `rtk git add go.mod internal/config && rtk git commit -m "feat: add validated service configuration"`

### Task 2: Implement Bambuddy normalization and bearer authentication

**Files:**
- Create: `internal/notification/notification.go`
- Create: `internal/notification/notification_test.go`
- Create: `internal/httpapi/auth.go`
- Test: `internal/httpapi/auth_test.go`

**Interfaces:**
- `notification.BambuddyPayload` decodes JSON fields `title` and `message` while allowing additional fields.
- `notification.Notification` contains `Title` and `Body`.
- `notification.FromBambuddy(payload BambuddyPayload) (Notification, error)` trims and validates required text.
- `httpapi.BearerToken(r *http.Request) (string, bool)` extracts exactly one bearer token.

- [ ] **Step 1: Write failing normalization and authentication tests**

```go
func TestFromBambuddyMapsTitleAndMessage(t *testing.T) {
	got, err := FromBambuddy(BambuddyPayload{Title: "  Print failed ", Message: "  Nozzle error  "})
	if err != nil {
		t.Fatalf("FromBambuddy() error = %v", err)
	}
	if got != (Notification{Title: "Print failed", Body: "Nozzle error"}) {
		t.Fatalf("notification = %#v", got)
	}
}

func TestFromBambuddyRejectsBlankFields(t *testing.T) {
	for _, payload := range []BambuddyPayload{{Message: "body"}, {Title: "title"}, {Title: " ", Message: "body"}} {
		if _, err := FromBambuddy(payload); err == nil {
			t.Errorf("FromBambuddy(%#v) error = nil", payload)
		}
	}
}
```

```go
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
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/notification ./internal/httpapi`

Expected: FAIL because the normalization and auth helpers do not exist.

- [ ] **Step 3: Implement the minimal source-neutral value and helpers**

Use `strings.TrimSpace` for title and body. Return an error identifying the missing field. Decode the auth header using `strings.Fields`, require two fields, compare the scheme with `strings.EqualFold`, and reject an empty token.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `go test ./internal/notification ./internal/httpapi`

Expected: PASS.

- [ ] **Step 5: Commit the normalization unit**

Run: `rtk git add internal/notification internal/httpapi && rtk git commit -m "feat: normalize Bambuddy notifications"`

### Task 3: Implement the Bark client

**Files:**
- Create: `internal/bark/client.go`
- Test: `internal/bark/client_test.go`

**Interfaces:**
- `bark.Client` has `Send(ctx context.Context, notification notification.Notification) error`.
- `bark.NewClient(baseURL, deviceKey string, httpClient *http.Client) (*Client, error)` validates and normalizes the base URL.

- [ ] **Step 1: Write failing Bark client tests**

```go
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
```

Add these failure tests:

```go
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
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/bark`

Expected: FAIL because `Client` and `Send` do not exist.

- [ ] **Step 3: Implement the minimal client**

Join the configured base URL with `/push`, encode `{body, device_key, title}` as JSON, set `Content-Type: application/json`, use the caller context, close the response body, and return an error for any status outside `200`–`299`. Do not include the device key or notification body in error text.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `go test ./internal/bark`

Expected: PASS.

- [ ] **Step 5: Commit the Bark client**

Run: `rtk git add internal/bark && rtk git commit -m "feat: add Bark push client"`

### Task 4: Build the HTTP API and end-to-end handler tests

**Files:**
- Create: `internal/httpapi/server.go`
- Test: `internal/httpapi/server_test.go`

**Interfaces:**
- `httpapi.Server` exposes `Handler() http.Handler`.
- `httpapi.NewServer(expectedToken string, sender Sender, requestTimeout time.Duration) *Server`.
- `httpapi.Sender` is `Send(ctx context.Context, notification notification.Notification) error`.

- [ ] **Step 1: Write failing handler tests**

Cover these exact cases with `httptest` and a recording sender:

```go
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
}
```

Add focused tests for `401` without/with a wrong token, `400` malformed JSON and blank fields, `502` when the sender returns an error, `405` for `GET /webhook/bambuddy`, and `200` for `GET /healthz` without a sender call.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/httpapi`

Expected: FAIL because the server handler does not exist.

- [ ] **Step 3: Implement the handler**

Route only the two documented paths. Authenticate the webhook before decoding its body. Decode one JSON object with a bounded body reader, reject trailing JSON values, normalize through `notification.FromBambuddy`, call the sender with a timeout context, and write the status codes from the design. Keep response bodies short and generic.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `go test ./internal/httpapi`

Expected: PASS.

- [ ] **Step 5: Commit the HTTP API**

Run: `rtk git add internal/httpapi && rtk git commit -m "feat: add authenticated Bambuddy webhook API"`

### Task 5: Add executable startup, graceful shutdown, and systemd deployment

**Files:**
- Create: `cmd/bambark/main.go`
- Create: `cmd/bambark/main_test.go`
- Create: `deploy/bambark.service`

**Interfaces:**
- `cmd/bambark/main.go` loads `config.Config`, constructs the Bark client and API server, starts an `http.Server`, and shuts it down on `SIGINT` or `SIGTERM`.
- `newServer(cfg config.Config, client *http.Client) (*http.Server, error)` is kept small enough for startup tests.

- [ ] **Step 1: Write a failing startup-construction test**

Test that a valid `config.Config` produces a server with the configured listen address and a non-nil handler. Test that an invalid Bark URL returns an error without starting a listener.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./cmd/bambark`

Expected: FAIL because startup construction does not exist.

- [ ] **Step 3: Implement startup and shutdown**

Use an `http.Client` with a 10-second timeout, construct `bark.Client`, create the API handler, and start the server. Register signal handling with `signal.NotifyContext`; on cancellation, call `Shutdown` with a 5-second grace period. Log startup and shutdown events without printing secrets.

- [ ] **Step 4: Add the systemd unit**

Create a unit that runs `/usr/local/bin/bambark`, loads `/etc/bambark/bambark.env`, uses `User=bambark`, enables `Restart=on-failure`, and starts after `network-online.target`. Do not put credentials directly in the unit.

- [ ] **Step 5: Run tests and compile the binary**

Run: `go test ./...`

Expected: PASS.

Run: `go build ./cmd/bambark`

Expected: exit code 0 and a compiled `bambark` binary.

- [ ] **Step 6: Commit the executable and service unit**

Run: `rtk git add cmd deploy && rtk git commit -m "feat: add persistent Bambark service"`

### Task 6: Write deployment documentation and example configuration

**Files:**
- Create: `README.md`
- Create: `deploy/bambark.env.example`

- [ ] **Step 1: Write the documentation examples**

Document these commands with placeholders only:

```sh
go build -o bambark ./cmd/bambark
sudo install -m 0755 bambark /usr/local/bin/bambark
sudo install -d -m 0750 /etc/bambark
sudo install -m 0640 deploy/bambark.env.example /etc/bambark/bambark.env
sudo systemctl enable --now bambark.service
curl http://127.0.0.1:8080/healthz
```

Document the Bambuddy URL as `http://<lxc-address>:8080/webhook/bambuddy` and show a `curl` request containing the bearer token placeholder. Explain that the wrapper assumes the trusted internal network and does not terminate TLS.

- [ ] **Step 2: Check documentation for leaked secrets and incorrect paths**

Run: `rtk grep -n 'device-key\|secret\|token' README.md deploy/bambark.env.example`

Expected: only clearly marked placeholders such as `replace-with-a-secret` appear.

- [ ] **Step 3: Add the example environment file**

Include `LISTEN_ADDR=:8080`, `BARK_URL=http://127.0.0.1:8081`, `BARK_DEVICE_KEY=replace-with-bark-device-key`, and `WEBHOOK_BEARER_TOKEN=replace-with-a-long-random-token`.

- [ ] **Step 4: Run the full verification suite**

Run: `go test ./...`

Expected: PASS with zero failing tests.

Run: `go vet ./...`

Expected: no diagnostics.

Run: `go build ./cmd/bambark`

Expected: exit code 0.

- [ ] **Step 5: Commit documentation**

Run: `rtk git add README.md deploy/bambark.env.example && rtk git commit -m "docs: add Debian deployment instructions"`

### Task 7: Final requirements verification

**Files:**
- Inspect: all repository files and git history

- [ ] **Step 1: Review the implementation against the design**

Confirm there is exactly one input source endpoint, exactly one configured Bark device, bearer authentication on the webhook, no database or queue, and an unauthenticated health endpoint.

- [ ] **Step 2: Run the complete verification commands**

Run:

```sh
go test ./...
go vet ./...
go build ./cmd/bambark
rtk git diff --check HEAD~6..HEAD
rtk git status --short
```

Expected: all commands succeed; the final status contains no unintended generated binary or untracked files.

- [ ] **Step 3: Inspect the final diff**

Run: `rtk git log --oneline --decorate -8 && rtk git diff HEAD~6..HEAD --stat`

Confirm the commits contain only the wrapper, tests, deployment unit, example configuration, README, and approved design/plan documents.
