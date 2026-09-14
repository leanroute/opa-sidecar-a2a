# Changelog

## [Unreleased]

- Nothing yet.

## v0.1.0-alpha — 2026-09-14

Initial reference implementation.

**Added**
- HTTP `/authorize` endpoint (POST, JSON in / JSON out).
- Embedded OPA policy engine, hot-reload on SIGHUP.
- `/healthz` and `/readyz` probes.
- Reference chain-verification helpers (`policies/chain_auth.rego`).
- Reference policies:
  - `example_delegation.rego`: baseline chain-verified allow
  - `example_scoped_tool.rego`: enforce tool-specific scopes
  - `example_time_bound.rego`: business-hours restriction
  - `example_obligations.rego`: return obligations alongside allow
- Worked planner-executor demo under `examples/planner-executor/`.
- Docs: architecture, wire protocol, policy-authoring guide.
- CI: go build/test/vet/gofmt/golangci-lint + OPA policy syntax check.
- Docker path (`Dockerfile`, `docker-compose.yml`) so Go install isn't required to try the demo.
- PowerShell `make.ps1` for Windows users without GNU make.

**Fixed (pre-release smoke)**
- `future.keywords.if` imports missing from four policy files.
- Demo ed25519 seeds were 31 bytes; corrected to 32 via `seedFromLabel()`.
- Policies now check `chain_terminates_in(input.callee.id)` (recipient), not caller.

**Known limitations (all on v0.2+ roadmap)**
- Sidecar-side signature verification. v0.1 relies on the caller marking `sig_verified` on each grant. v0.2 will verify against the trust dir authoritatively.
- No revocation lists.
- No JWKS auto-fetch for trust anchors.
- No metrics or tracing.
- No decision caching.
- No Envoy ext_authz gRPC support.
- No W3C VC 2.0 credential format.
