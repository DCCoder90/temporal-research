FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY . .
RUN go mod tidy \
 && CGO_ENABLED=0 GOOS=linux go build -o /fail-starter ./fail-starter/

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /fail-starter /fail-starter

ENTRYPOINT ["/fail-starter"]
