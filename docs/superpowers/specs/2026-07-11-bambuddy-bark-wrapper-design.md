# Bambuddy-to-Bark Webhook Wrapper Design

## Goal

Build a small long-lived service that receives Bambuddy notification webhooks inside The Old Bakery network and forwards them as push notifications to one configured Bark device.

The flow is:

```text
Bambuddy --HTTP webhook--> Bambark wrapper --Bark HTTP API--> Bark --APNS--> Bark app
```

The initial service supports only Bambuddy as an input source and one Bark destination. It should be simple to deploy in a Debian LXC and leave a clear, small seam for a future second input source without introducing multi-user or multi-destination behavior now.

## Scope

### In scope

- A long-running HTTP service.
- `POST /webhook/bambuddy` as the only notification input endpoint.
- Bearer-token authentication for the webhook endpoint.
- Parsing Bambuddy's standardized JSON webhook payload.
- Mapping Bambuddy `title` and `message` to Bark `title` and `body`.
- One Bark server URL and one Bark device key configured by environment variables.
- Synchronous forwarding to Bark's `POST /push` API.
- A lightweight unauthenticated `GET /healthz` endpoint.
- HTTP status handling that allows Bambuddy to retry when Bark is unavailable.
- A Debian/systemd deployment guide.

### Out of scope

- User accounts, per-user credentials, or multiple tenants.
- Multiple Bark devices or selectable destinations.
- Persistent queues, databases, retry workers, or delivery history.
- Alternate notification sources.
- Alternate delivery targets.
- TLS termination inside the service; network TLS/reverse proxy policy remains an infrastructure concern.

## Runtime and deployment

The service will be implemented in Go using the standard library. It will compile to a single binary, minimizing runtime dependencies in the Debian LXC.

The service will run under systemd with:

- an unprivileged service user;
- environment-based configuration, preferably loaded from a root-readable environment file;
- automatic restart after process failure;
- a configurable listen address, defaulting to a local HTTP port suitable for an LXC deployment.

The service does not store application state. “Persistent” means the process is continuously managed by systemd and restarts after failure; notifications are delivered synchronously rather than queued.

## Configuration

The following environment variables are required unless a documented default is given:

| Variable | Purpose |
| --- | --- |
| `LISTEN_ADDR` | HTTP bind address, with a documented default such as `:8080`. |
| `BARK_URL` | Base URL of the Bark server, for example `http://127.0.0.1:8080`. |
| `BARK_DEVICE_KEY` | The single configured Bark device key. |
| `WEBHOOK_BEARER_TOKEN` | Secret token required on Bambuddy webhook requests. |

The implementation should validate required configuration at startup and fail clearly rather than starting partially configured.

Secrets must not be logged. The example environment file should use placeholders and should not contain real credentials.

## HTTP contract

### `POST /webhook/bambuddy`

Required header:

```http
Authorization: Bearer <WEBHOOK_BEARER_TOKEN>
Content-Type: application/json
```

The request body is Bambuddy's standardized webhook object. The wrapper requires:

```json
{
  "title": "Print failed",
  "message": "The printer reported an error",
  "timestamp": "2026-07-11T12:00:00Z",
  "source": "Bambuddy",
  "event": "print_failed"
}
```

Only `title` and `message` are required for forwarding. Other Bambuddy fields are accepted and ignored by the initial adapter so the wrapper remains compatible with event-specific fields.

Responses:

- `202 Accepted` after Bark accepts the notification.
- `400 Bad Request` for malformed JSON or missing/blank required fields.
- `401 Unauthorized` for a missing or incorrect bearer token.
- `405 Method Not Allowed` for non-POST requests to the webhook path.
- `502 Bad Gateway` when the Bark request cannot be completed successfully or Bark rejects it.

The service should use bounded request and outbound HTTP timeouts. It should not report success before the Bark request receives a successful response.

### `GET /healthz`

Returns `200 OK` with a small plain-text or JSON response when the HTTP process is alive. It does not call Bark and does not require authentication, allowing systemd or local monitoring to check process health.

## Internal boundaries

The code should keep these responsibilities separate:

1. HTTP authentication and request handling.
2. Bambuddy payload decoding and validation.
3. An internal notification value containing the normalized title and body.
4. A Bark client that converts that value into Bark's `POST /push` request.
5. Configuration and process startup.

The Bambuddy adapter is the only source adapter initially. Future sources can add another adapter that produces the same internal notification value without changing the Bark client or authentication primitives.

## Error handling and delivery semantics

Delivery is synchronous and at-most-one outbound attempt per inbound request. If the wrapper cannot reach Bark or Bark returns an unsuccessful response, the wrapper returns `502` so Bambuddy can apply its own webhook retry behavior.

Because a caller may retry after an ambiguous network failure, duplicate push notifications are possible. The initial service will not add idempotency storage; avoiding a database is part of the deliberately small scope.

Logs should include request outcome, endpoint, and Bark status where useful, but must not include bearer tokens, Bark device keys, or full notification bodies by default.

## Testing strategy

Tests will cover:

- valid bearer authentication;
- missing and invalid bearer authentication;
- malformed JSON;
- missing or blank `title`/`message`;
- successful Bambuddy-to-Bark mapping;
- Bark failure and timeout propagation as `502`;
- health endpoint behavior;
- startup rejection for missing required configuration.

The Bark HTTP client will use an injectable HTTP client or test server so tests exercise real request encoding and response handling without contacting an external Bark instance.

## Operational documentation

The repository will include a README covering:

- building the binary;
- creating the environment file;
- installing the systemd unit;
- configuring Bambuddy's webhook URL and bearer token;
- testing health and notification delivery with `curl`;
- viewing service logs;
- the expected network assumptions and the absence of built-in TLS.

The Bark API shape used by the client follows Bark's documented JSON endpoint: `POST /push` with `device_key`, `title`, and `body` fields.
