# SMSpit - The Mailpit of SMS Testing
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o smspit .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /app/smspit .

# Create data directory
RUN mkdir -p /data

# Expose ports
EXPOSE 8080 9080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Environment defaults
ENV SMSPIT_DB_PATH=/data/smspit.db
ENV SMSPIT_WEB_PORT=8080
ENV SMSPIT_API_PORT=9080

# Run
CMD ["./smspit"]

