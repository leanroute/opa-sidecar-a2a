# Multi-stage build for opa-sidecar-a2a.
#
# Build stage compiles the Go binary; runtime stage ships a minimal
# distroless image with just the binary + reference policies.
#
# Build:  docker build -t leanroute/opa-sidecar-a2a:dev .
# Run:    docker run -p 8181:8181 leanroute/opa-sidecar-a2a:dev

# ─── build stage ────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependencies separately from source to keep rebuilds fast.
COPY go.mod ./
# go.sum is regenerated when this file changes; ok to be missing on first
# build (the go mod download will produce it).
COPY go.su[m] ./

RUN go mod download

COPY . .

# Static build, no cgo — makes the resulting binary work in distroless
# and scratch base images.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=$(cat CHANGELOG.md | grep -Eo 'v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9]+)?' | head -1)" \
    -o /out/opa-sidecar \
    ./cmd/opa-sidecar

# ─── runtime stage ──────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/opa-sidecar /app/opa-sidecar
COPY policies /app/policies

# Trust dir is intentionally left empty in the base image. Mount your
# own trust anchors at /app/trust when running in production.
COPY --chown=nonroot:nonroot examples/planner-executor/trust /app/trust

EXPOSE 8181

USER nonroot:nonroot

ENTRYPOINT ["/app/opa-sidecar"]
CMD ["--listen", ":8181", "--policy-dir", "/app/policies", "--trust-dir", "/app/trust"]
