FROM golang:1.25-alpine3.22 AS build_deps

RUN apk add --no-cache git
WORKDIR /workspace
ENV GO111MODULE=on

COPY go.mod go.sum ./
RUN go mod download

# ---- Build stage ----
FROM build_deps AS build
COPY . .

# CGO disabled => static binary by default, no extldflags needed
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o webhook -ldflags="-w" .

# ---- Final stage ----
FROM ghcr.io/dns-lexicon/dns-lexicon:3.23.2

LABEL \
  org.opencontainers.image.authors="Andreas Senfter" \
  org.opencontainers.image.url="https://github.com/asenfter/cert-manager-webhook-dns-lexicon" \
  org.opencontainers.image.title="cert-manager-webhook-dns-lexicon" \
  org.opencontainers.image.description="Webhook service for cert-manager built on top of dns-lexicon for automated DNS updates" \
  org.opencontainers.image.base.name="ghcr.io/dns-lexicon/dns-lexicon:3.23.2"

RUN apt-get update \
 && apt-get install -y ca-certificates wget \
 && rm -rf /var/lib/apt/lists/* \
 && adduser --disabled-password --gecos "" --uid 1000 appuser

USER appuser

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
 CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1
