# ========== Stage 1: Builder ==========
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Step 1: Copy only dependency files first for layer caching
# (docker will cache this layer unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

# Step 2: Copy all source code and build the server binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/server ./cmd/server

# ========== Stage 2: Runtime ==========
FROM alpine:latest AS runtime

# Install ca-certificates for HTTPS outbound (if any)
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the compiled binary from builder
COPY --from=builder /app/server ./server

# Copy static frontend files (the Go server uses http.FileServer from current dir)
COPY --from=builder /build/index.html ./index.html
COPY --from=builder /build/style.css ./style.css
COPY --from=builder /build/auth.js ./auth.js
COPY --from=builder /build/assets ./assets
COPY --from=builder /build/ui-screens ./ui-screens
COPY --from=builder /build/js ./js

# Create uploads directory for runtime data
RUN mkdir -p /app/uploads && chown -R appuser:appgroup /app

# Create TLS directory for certificates (mounted via volume)
RUN mkdir -p /app/tls && chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose the application ports (HTTP and HTTPS)
EXPOSE 8080 8443

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the server
ENTRYPOINT ["./server"]
