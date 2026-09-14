# Builds and runs the planner-executor worked demo inside a container.
# Intended to be composed alongside the sidecar via docker-compose.yml.

FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY go.su[m] ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/planner  ./examples/planner-executor/planner \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/executor ./examples/planner-executor/executor

# ─── runtime ────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache bash

WORKDIR /app

COPY --from=builder /out/planner  /app/planner
COPY --from=builder /out/executor /app/executor
COPY examples/planner-executor/run-demo-docker.sh /app/run-demo.sh

RUN chmod +x /app/run-demo.sh

ENTRYPOINT ["/app/run-demo.sh"]
