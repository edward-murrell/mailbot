# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mailbot ./cmd/mailbot

# Final stage — minimal, runs as nonroot (uid 65532)
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /mailbot /mailbot

EXPOSE 8080

ENTRYPOINT ["/mailbot"]
