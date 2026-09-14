# opa-sidecar-a2a — dev commands.
#
# Zero-config: assumes Go 1.22+ on PATH. No extra tools required for `build` or
# `test`; `lint` needs `golangci-lint` (install: https://golangci-lint.run/usage/install/).

BINARY := bin/opa-sidecar
CMD    := ./cmd/opa-sidecar
POLICY_DIR := ./policies

.PHONY: build test lint run demo clean fmt vet

## build : compile the sidecar into bin/opa-sidecar
build:
	@mkdir -p bin
	go build -o $(BINARY) $(CMD)

## test : run unit tests with race detector
test:
	go test -race -count=1 ./...

## lint : run golangci-lint (install separately)
lint:
	golangci-lint run ./...

## fmt : gofmt the tree
fmt:
	gofmt -s -w .

## vet : go vet the tree
vet:
	go vet ./...

## run : start the sidecar against ./policies on :8181
run: build
	$(BINARY) --policy-dir $(POLICY_DIR) --listen :8181

## demo : run the planner-executor worked example
demo: build
	@echo "starting sidecar in background..."
	@$(BINARY) --policy-dir $(POLICY_DIR) --listen :8181 & \
	  SIDECAR_PID=$$!; \
	  sleep 1; \
	  cd examples/planner-executor && ./run-demo.sh; \
	  DEMO_RC=$$?; \
	  kill $$SIDECAR_PID 2>/dev/null; \
	  exit $$DEMO_RC

## clean : delete build artifacts
clean:
	rm -rf bin/ coverage.txt

## help : this message
help:
	@grep -E '^## ' Makefile | sed -e 's/## //'
