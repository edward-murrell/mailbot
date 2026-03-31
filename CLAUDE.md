# mailbot

A Go HTTP daemon exposing a REST endpoint for contact form submissions. Clients (React, Vue, or similar SPAs) POST JSON to the endpoint. On each valid submission, the daemon sends an email via authenticated SMTP and writes a plain-text copy to a local directory.

## Architecture

- Single binary, single REST endpoint: `POST /contact`
- No TLS — a reverse proxy (nginx, Caddy, Traefik, etc.) handles TLS termination
- Runs as a Docker container; all configuration via environment variables
- Two side effects per valid submission, both attempted: send email + write file
  - A failure in one should not suppress the other; log both outcomes
- Dependency injection throughout — no package-level singletons or global state

## Project layout

Follow https://github.com/golang-standards/project-layout strictly:

```
mailbot/
├── cmd/
│   └── mailbot/
│       └── main.go          # wires dependencies, starts server
├── internal/
│   ├── config/              # env var parsing and validation
│   ├── handler/             # HTTP handler (thin — delegates to service layer)
│   ├── mailer/              # SMTP sending logic
│   ├── store/               # submission file writing
│   ├── middleware/          # rate limiting, logging, recovery
│   └── submission/          # Submission type and field validation
├── Dockerfile
├── docker-compose.yml       # for local development
├── CLAUDE.md
└── go.mod
```

## REST endpoint

### `POST /contact`

**Content-Type:** `application/json`

**Request body fields:**

| Field   | Type   | Required |
|---------|--------|----------|
| name    | string | No       |
| email   | string | Conditional — email or phone required |
| phone   | string | Conditional — email or phone required |
| subject | string | Conditional — subject or body or reason required |
| body    | string | Conditional — subject or body or reason required |
| reason  | string | Conditional — subject or body or reason required |

**Validation rules:**
- At least one of `email` or `phone` must be non-empty
- At least one of `subject`, `body`, or `reason` must be non-empty
- If `email` is provided it must be a valid email address format

**Success response:** `202 Accepted`
```json
{"ok": true}
```

**Error responses:**
- `400 Bad Request` — validation failure, includes field-level detail:
  ```json
  {"ok": false, "errors": {"email": "email or phone is required"}}
  ```
- `405 Method Not Allowed` — any method other than POST
- `429 Too Many Requests` — rate limit exceeded
- `500 Internal Server Error` — after attempting both side effects

## Configuration (environment variables)

| Variable              | Description                                          | Default        |
|-----------------------|------------------------------------------------------|----------------|
| `LISTEN_ADDR`         | Address and port to bind                             | `:8080`        |
| `SMTP_HOST`           | SMTP server hostname                                 | required       |
| `SMTP_PORT`           | SMTP server port                                     | `587`          |
| `SMTP_USER`           | SMTP username                                        | required       |
| `SMTP_PASS`           | SMTP password                                        | required       |
| `SMTP_FROM`           | Envelope from address                                | required       |
| `SMTP_TO`             | Destination address for contact form submissions     | required       |
| `SMTP_STARTTLS`       | Use STARTTLS (`true`/`false`)                        | `true`         |
| `SMTP_ENABLED`        | Set to `false` to disable SMTP (local dev/testing)  | `true`         |
| `STORAGE_DIR`         | Directory to write submission text files into        | required       |
| `RATE_LIMIT_INTERVAL` | Minimum seconds between submissions per IP           | `5`            |
| `LOG_LEVEL`           | `debug`, `info`, `warn`, `error`                     | `info`         |

Fail fast at startup if any required variable is absent (required variables are skipped when their feature is disabled, e.g. SMTP vars when `SMTP_ENABLED=false`).

## File storage

Each submission is written as a UTF-8 plain-text file in `STORAGE_DIR`.

Filename format: `YYYYMMDD-HHMMSS-<random-6-chars>.txt` (UTC)

File contents:
```
Date:    2026-03-31T14:05:22Z
Name:    Jane Smith
Email:   jane@example.com
Phone:   +61 400 000 000
Subject: Website inquiry
Reason:  Support
Body:
Hello, I have a question about...
```

Create `STORAGE_DIR` on startup if it does not exist.

## Code conventions

### Design principles
- **Dependency injection:** construct dependencies in `main.go` and pass them down; no `init()` side effects
- **Pure functions:** validation, formatting, and serialisation logic should be pure functions with no side effects, easy to unit test in isolation
- **Immutable data:** the `Submission` type (in `internal/submission`) should be treated as immutable after construction — no setters, no mutation after the handler creates it
- **Interface-driven:** `mailer` and `store` packages expose interfaces; `main.go` wires concrete implementations. This enables a no-op mailer for `SMTP_ENABLED=false` without conditionals scattered through the code.

### Style
- Structured logging with `log/slog` (stdlib, no external logging dependency)
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- HTTP handlers are thin — decode, validate, call service, encode response
- Use `net/smtp` from stdlib; use `gopkg.in/gomail.v2` only if STARTTLS handling with stdlib becomes unwieldy
- Pass `context.Context` as the first argument through to SMTP and file operations

## Middleware

Middleware lives in `internal/middleware/` and is composed in `cmd/mailbot/main.go`.

### Rate limiting
- Per-IP, token bucket or sliding window — one submission per `RATE_LIMIT_INTERVAL` seconds (default 5)
- Returns `429 Too Many Requests` with a `Retry-After` header when exceeded
- IP extraction must handle `X-Forwarded-For` / `X-Real-IP` headers (set by the upstream reverse proxy)

### Other middleware
- Request logging (method, path, status, duration)
- Panic recovery → `500`

## Testing

- Unit test validation logic in `internal/submission` — the conditional required-field rules are the trickiest part
- Integration tests for the HTTP handler use `net/http/httptest`
- Do not mock the filesystem — use `t.TempDir()` for real file I/O
- Test the no-op mailer path using `SMTP_ENABLED=false` configuration
- SMTP integration tests against a local stub (e.g. MailHog) or skip in short mode (`testing.Short()`)

Run tests: `go test ./...`

## Docker

- Multi-stage build: build stage `golang:1.24-alpine`, final stage `gcr.io/distroless/static` or `alpine:3`
- `STORAGE_DIR` is a Docker volume mount so submissions survive restarts
- Container runs as a non-root user
- Expose port `8080`
- No TLS certificates in the container

## Non-goals

- No authentication on the endpoint (the SPA client is public-facing)
- No database — file storage only
- No server-side form rendering
- No email HTML templating — plain text is sufficient
- No admin UI or submission viewer
