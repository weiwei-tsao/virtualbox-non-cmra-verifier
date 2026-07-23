# =============================================================
# Stage 1: Build — compile static Go binary
# =============================================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy module files first to leverage Docker layer caching.
# go.mod declares go 1.25.3, so golang:1.25-alpine is required.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

# Copy full source (build context is the repo root).
COPY apps/api/ .

# CGO_ENABLED=0: static binary, no libc dependency.
# Alpine uses musl; a CGO binary built elsewhere would not run.
# -ldflags="-w -s": strip debug symbols, reduces binary size ~30%.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o server \
    ./cmd/server/main.go

# =============================================================
# Stage 2: Runtime — minimal Alpine image with Chromium
# =============================================================
FROM alpine:3.21

WORKDIR /app

# chromedp runtime dependencies:
#   chromium       — headless Chrome binary for iPost1 crawler
#   nss            — NSS crypto library required by Chromium TLS
#   freetype       — font rendering (Chromium page layout)
#   harfbuzz       — Unicode text shaping (required by Chromium)
#   ca-certificates — system root certs for HTTPS (Smarty API, Firestore)
#   tzdata          — timezone data (revalidation checker runs at 02:00 UTC)
RUN apk update && apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    tzdata && \
    rm -rf /var/cache/apk/*

# Alpine installs Chromium at /usr/bin/chromium-browser, not /usr/bin/chromium.
# chromedp's ExecAllocator uses this path to find the browser binary.
ENV CHROME_PATH=/usr/bin/chromium-browser

# Copy the static binary from the build stage.
COPY --from=builder /app/server .

# Run as a non-root user. Safe because --no-sandbox (injected via
# CHROME_NO_SANDBOX=true) removes the kernel sandbox requirement.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    chown appuser:appgroup /app/server
USER appuser

EXPOSE 8080

CMD ["./server"]
