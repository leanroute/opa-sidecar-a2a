# Architecture

## Deployment model

The sidecar is a single Go binary that runs alongside an agent or a
gateway. It exposes exactly one production endpoint (`POST /authorize`)
plus health probes (`/healthz`, `/readyz`). It holds no persistent state;
every decision is a pure function of `(request input, loaded policies,
loaded trust anchors)`.

Two common deployment shapes:

**Colocated with the callee agent.** The executor agent's pod includes
the sidecar as a second container. The agent calls
`http://localhost:8181/authorize` before every action. This is the
Envoy-style sidecar pattern applied to authorization instead of traffic.
Latency: <5ms in-pod.

**Centralized behind a gateway.** A single sidecar deployment sits behind
the LLM gateway. Every A2A call the gateway proxies triggers an
`/authorize` roundtrip. Higher latency but simpler operationally,
especially in FaaS or serverless deployments where injecting sidecars is
awkward.

Both are supported by the same binary. Choose based on your ops model,
not a technical constraint.

## Request path

```
1. Caller (planner) constructs delegation chain.
2. Caller sends request to callee (executor) over HTTP or A2A wire.
3. Callee's sidecar (or gateway):
   a. Reads the request input + chain.
   b. Verifies every signature against the trust dir.
   c. Marks each grant `sig_verified: true` or `false`.
   d. Passes the enriched input to the OPA engine.
   e. OPA evaluates `data.a2a.authorize.result`.
   f. Returns { allow, reason, reason_code, obligations }.
4. Callee honors the decision:
   - allow → proceed with the action, apply any obligations.
   - deny → return 403 with the reason to the caller.
```

The sidecar itself does not enforce anything. It advises. The caller
(agent or gateway) is responsible for honoring the returned decision
and enforcing obligations.

## Policy loading

At startup the sidecar reads every `.rego` file under `--policy-dir`
(default `./policies`) and compiles them into a single query prepared
against `data.a2a.authorize.result`. Multiple `.rego` files can
contribute to the same package; OPA merges rules from all files. If two
files disagree (one says allow, one says deny) OPA's default-value
semantics apply — whichever rule matches last wins, so authors should
avoid overlapping rules and use `default result := ...` at the top of
each policy file to make the deny-by-default explicit.

**Hot reload:** SIGHUP re-reads and recompiles. If compilation fails the
existing policy set is preserved and an error is logged. Callers see no
disruption during reload — pending requests complete against the old
policy set.

## Trust anchors

Public keys of principals whose signatures the sidecar accepts live under
`--trust-dir` (default `./trust`) as PEM files. Filename convention:
`<kid>.pem`. The kid header must match what appears in the delegation
grant.

Trust anchor management is deliberately spartan in v0.1:

- No revocation. If a key is compromised, remove the PEM and SIGHUP.
- No rotation with grace period. Overlap two PEMs for the transition.
- No auto-fetch from a JWKS endpoint. That's on the v0.2 roadmap.

The reason for the simplicity: v0.1 is meant to be understood
end-to-end by anyone reading the source in an afternoon. Revocation
and rotation are real features and will land, but adding them to v0.1
would make the reference implementation hard to reason about.

## Failure modes

| Failure | Sidecar behavior | Reason code |
| ------- | ---------------- | ----------- |
| Malformed JSON | 400 + `json_parse_error` | — |
| Missing `input` object | 400 + `missing_input` | — |
| Body > 1 MiB | 413 + `body_too_large` | — |
| Signature invalid | Grant marked `sig_verified: false`; policy returns deny | `chain_invalid` |
| Grant expired | Same as above | `chain_invalid` |
| Chain hops don't link | Same as above | `chain_invalid` |
| Chain doesn't cover action | Deny | `chain_missing_scope` |
| Chain doesn't terminate in caller | Deny | `chain_wrong_delegate` |
| Policy returns undefined | Deny | `policy_undefined` |
| Policy returns unexpected shape | Deny | `policy_shape_error` |
| OPA eval error (bug in policy) | 500 + `policy_eval_error` | — |

The design principle: **never allow by default**. Every failure mode
either returns a deny with a machine-readable reason code, or a 4xx/5xx
error that a client cannot mistake for an allow.

## What v0.1 does not do

Explicitly out of scope for the reference implementation. Roadmap items
in the README cover several of these.

- No revocation lists or CRL support.
- No JWKS endpoint fetching for trust anchors.
- No sidecar-to-sidecar gossip.
- No caching of previous decisions. Every request evaluates from scratch.
- No metrics endpoint. Add OTEL in v0.2.
- No policy authoring UI. Rego is the language.
- No W3C VC 2.0 credential format support. Custom grant shape only in v0.1.
