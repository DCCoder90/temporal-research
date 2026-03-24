FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy the full module. go mod tidy resolves and pins all dependencies,
# generating go.sum if it does not yet exist (first-time bootstrap).
# After the first build, commit go.mod + go.sum for reproducible local builds.
COPY . .
RUN go mod tidy \
 && CGO_ENABLED=0 GOOS=linux go build -o /starter ./starter/

# ── Runtime image ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /starter /starter

ENTRYPOINT ["/starter"]
