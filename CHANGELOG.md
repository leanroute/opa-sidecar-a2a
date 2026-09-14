# Changelog

## [Unreleased]

- Nothing yet.

## v0.1.0-alpha — 2026-09-13

Initial reference implementation.

**Added**
- HTTP `/authorize` endpoint (POST, JSON in / JSON out).
- Embedded OPA policy engine, hot-reload on SIGHUP.
- `/healthz` and `/readyz` probes.
- Reference chain-verification helpers (`policies/chain_auth.rego`).
- Reference policies:
  - `example_delegation.rego` — baseline chain-verified allow
  - `example_scoped_tool.rego` — enforce tool-specific scopes
  - `example_time_bound.rego` — business-hours restriction
  - `example_obligations.rego` — return obligations alongside allow
- Worked planner-executor demo under `examples/planner-executor/`.
- Docs: architecture, wire protocol, policy-authoring guide.
- CI: go build/test/vet/gofmt/golangci-lint + OPA policy syntax check.

**Known limitations (all on v0.2+ roadmap)**
- No revocation lists.
- No JWKS auto-fetch for trust anchors.
- No metrics or tracing.
- No decision caching.
- No Envoy ext_authz gRPC support.
- No W3C VC 2.0 credential format.
