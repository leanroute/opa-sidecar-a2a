# Contributing

Thanks for looking. This is a reference implementation, so contributions
that keep the codebase small and understandable are prioritized over
contributions that add capability.

## Workflow

1. Open an issue describing the problem or feature. Non-trivial changes
   without an issue are unlikely to merge, not out of gatekeeping but
   because we probably need to talk about the shape first.
2. Fork, branch, PR against `main`.
3. Every PR must pass CI (build, tests, gofmt, vet, golangci-lint, OPA
   policy syntax check).
4. Every code change ships with a test. Every policy change ships with
   a fixture under `tests/rego/`.
5. `CHANGELOG.md` entry required, top of file.

## Especially welcome

- Additional reference Rego policies for common delegation patterns.
- Non-Go client examples (Python, TypeScript, Rust).
- Integration examples with real A2A implementations (Google A2A SDK,
  Kagent, custom shapes).
- Hardening findings — this is a reference implementation, not a
  hardened production build. If you find something that would break
  under adversarial load, please report it.
- Docs improvements. Especially: places where the docs make an
  assumption that isn't spelled out.

## Not welcome (yet)

- New wire protocols beyond HTTP+JSON. Envoy ext_authz gRPC is on the
  roadmap for v0.2 — please wait rather than adding it early.
- New credential formats beyond the v0.1 grant shape. W3C VC 2.0 is on
  the v0.3 roadmap.
- Metrics endpoints. OpenTelemetry integration is on the v0.2 roadmap;
  early one-off metric exporters will be closed.
- Sidecar-to-sidecar communication of any kind. v0.1 is stateless by
  design; adding cross-sidecar state is a v0.2 conversation.

## License

By contributing you agree that your work is licensed under the MIT
License (see `LICENSE`).
