# mailbot

A Go HTTP daemon that exposes a single REST endpoint for contact form submissions. SPA clients (React, Vue, etc.) POST JSON to the endpoint; the daemon sends an email via authenticated SMTP and saves a plain-text copy of the submission to disk.

Designed to run as a Docker container behind a TLS-terminating reverse proxy.

## Quick start

```bash
# Local development — SMTP disabled, submissions saved to a Docker volume
docker compose up

# Submit a test message
curl -s -X POST http://localhost:8080/contact \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","subject":"Hello","body":"Just testing"}'
```

## Endpoint

### `POST /contact`

Accepts `application/json`.

| Field   | Type   | Required |
|---------|--------|----------|
| name    | string | No       |
| email   | string | Conditional — email **or** phone required |
| phone   | string | Conditional — email **or** phone required |
| subject | string | Conditional — subject **or** body **or** reason required |
| body    | string | Conditional — subject **or** body **or** reason required |
| reason  | string | Conditional — subject **or** body **or** reason required |

**Success — 202 Accepted**
```json
{"ok": true}
```

**Validation error — 400 Bad Request**
```json
{"ok": false, "errors": {"email": "email or phone is required"}}
```

**Rate limit — 429 Too Many Requests** (includes `Retry-After` header)

**Server error — 500 Internal Server Error**

## Configuration

All configuration is via environment variables.

| Variable              | Default   | Description |
|-----------------------|-----------|-------------|
| `LISTEN_ADDR`         | `:8080`   | Bind address |
| `SMTP_ENABLED`        | `true`    | Set `false` to skip sending (local dev) |
| `SMTP_HOST`           | required* | SMTP server hostname |
| `SMTP_PORT`           | `587`     | SMTP server port |
| `SMTP_USER`           | required* | SMTP username |
| `SMTP_PASS`           | required* | SMTP password |
| `SMTP_FROM`           | required* | Envelope from address |
| `SMTP_TO`             | required* | Destination address |
| `SMTP_SECURITY`       | `starttls`| `starttls` (port 587), `ssl` (port 465), `none` (no TLS, local dev) |
| `STORAGE_DIR`         | required  | Directory to write submission files |
| `RATE_LIMIT_INTERVAL` | `5`       | Seconds between allowed requests per IP |
| `LOG_LEVEL`           | `info`    | `debug` / `info` / `warn` / `error` |

\* Required only when `SMTP_ENABLED=true`.

## Development

### Prerequisites

- Go 1.22+
- Docker (for local dev with MailHog)

### Run locally without Docker

```bash
export SMTP_ENABLED=false
export STORAGE_DIR=/tmp/mailbot-submissions
go run ./cmd/mailbot
```

### Run tests

```bash
go test ./...

# Skip SMTP integration tests
go test -short ./...

# With race detector
go test -race ./...
```

### SMTP integration testing

Start MailHog via Docker Compose, then run with SMTP pointed at it:

```bash
docker compose up mailhog

SMTP_ENABLED=true \
SMTP_HOST=localhost \
SMTP_PORT=1025 \
SMTP_USER=test \
SMTP_PASS=test \
SMTP_FROM=test@example.com \
SMTP_TO=inbox@example.com \
SMTP_SECURITY=none \
STORAGE_DIR=/tmp/mailbot-submissions \
go run ./cmd/mailbot
```

View received messages at http://localhost:8025.

## Docker

Build and run directly:

```bash
docker build -t mailbot .

docker run \
  -e SMTP_ENABLED=false \
  -e STORAGE_DIR=/data \
  -v mailbot-data:/data \
  -p 8080:8080 \
  mailbot
```

## Production deployment

- Place behind a reverse proxy (nginx, Caddy, Traefik) that handles TLS.
- Mount a persistent volume at `STORAGE_DIR` so submissions survive restarts.
- Set `RATE_LIMIT_INTERVAL` to suit your expected traffic pattern.
- Ensure your reverse proxy sets `X-Real-IP` or `X-Forwarded-For` so rate limiting works correctly.
