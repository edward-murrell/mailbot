# mailbot

A Go HTTP daemon that exposes a single REST endpoint for contact form submissions. SPA clients (React, Vue, etc.) POST JSON to the endpoint; the daemon sends an email via authenticated SMTP and saves a plain-text copy of the submission to disk.

Designed to run as a Docker container behind a TLS-terminating reverse proxy.

## Why mailbot?

Static sites and SPAs have no backend to receive a contact requests. Existing solutions such as Formspree, Netlify Forms, require dependency on a third party. mailbot is a simple solution for self-hosters with minimal configuration and a small footprint.

Typical deployments:

- **Static sites** (Hugo, Astro, 11ty, plain HTML) needing a contact form without a CMS.
- **SPAs** (React, Vue, Svelte) on a CDN, where the form is the only server-side need.
- **Small business sites** where submissions go to the operator's inbox and stay on their own server, not a SaaS dashboard.
- **Internal tools** needing a lightweight "send feedback" or "request access" form.
- **Replacing PHP `mail()` scripts** during migration off shared hosting — same shape, but containerised and rate-limited.

The on-disk copy is an audit trail and a recovery path if SMTP bounces or a message is caught in spam.

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

Pre-built multi-arch images (`linux/amd64`, `linux/arm64`) are published to GitHub Container Registry on each release:

```
ghcr.io/edward-murrell/mailbot:0.1.0     # exact version — pin this for reproducible deploys
ghcr.io/edward-murrell/mailbot:0.1       # latest patch in the 0.1.x line
ghcr.io/edward-murrell/mailbot:latest    # most recent stable release
```

While the project is pre-1.0, minor version bumps (`0.1` → `0.2`) may include breaking changes. Track `:0.1` to receive patch fixes only, or pin `:0.1.0` for full lock-in.

```bash
docker pull ghcr.io/edward-murrell/mailbot:0.1.0

docker run \
  -e SMTP_ENABLED=false \
  -e STORAGE_DIR=/data \
  -v mailbot-data:/data \
  -p 8080:8080 \
  ghcr.io/edward-murrell/mailbot:0.1.0
```

Or build locally from source:

```bash
docker build -t mailbot .
```

## Production deployment

- Place behind a reverse proxy (nginx, Caddy, Traefik) that handles TLS.
- Mount a persistent volume at `STORAGE_DIR` so submissions survive restarts.
- Set `RATE_LIMIT_INTERVAL` to suit your expected traffic pattern.
- Ensure your reverse proxy sets `X-Real-IP` or `X-Forwarded-For` so rate limiting works correctly.

### nginx

```nginx
server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location = /contact {
        limit_except POST { deny all; }

        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```caddy
example.com {
    @contact {
        path   /contact
        method POST
    }
    reverse_proxy @contact 127.0.0.1:8080

    # ... other routes (static site, SPA, etc.)
}
```

Caddy handles TLS automatically via Let's Encrypt and sets `X-Forwarded-For` by default.
