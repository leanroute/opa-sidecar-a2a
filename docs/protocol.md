# Wire protocol

Everything the sidecar exposes on the wire. All endpoints are
HTTP/1.1 or HTTP/2 over any TLS-terminating proxy of your choice.

## POST /authorize

Ask the sidecar whether a proposed A2A call is authorized.

### Request

Headers:

| Header | Required | Value |
| ------ | -------- | ----- |
| `Content-Type` | yes | `application/json` |
| `Accept` | recommended | `application/json` |

Body: JSON object with a single top-level `input` key. The contents of
`input` are forwarded verbatim to the Rego engine, which means policies
bind to `input.caller`, `input.delegation_chain`, etc.

Canonical `input` schema:

| Field | Type | Required | Notes |
| ----- | ---- | -------- | ----- |
| `caller` | object | yes | `{id: string, type: "agent" | "user" | "service"}` |
| `callee` | object | yes | same shape as `caller` |
| `action` | string | yes | e.g., `invoke_tool`, `read_resource`, `emit_event` |
| `resource` | string | yes | Resource identifier the action targets |
| `delegation_chain` | array | yes | Ordered chain, from originating principal to caller |
| `context` | object | no | Free-form; passed to policies as `input.context` |
| `now` | string | no | RFC3339 timestamp; overrides wall-clock for deterministic tests |

Each element of `delegation_chain` is a grant:

| Field | Type | Required | Notes |
| ----- | ---- | -------- | ----- |
| `iss` | string | yes | Issuer principal id (whoever granted this hop) |
| `sub` | string | yes | Subject principal id (whoever received the grant) |
| `scope` | array of strings | yes | Actions the subject is authorized for |
| `exp` | integer | yes | Expiry, seconds since Unix epoch |
| `sig` | string | yes | Base64-encoded ed25519 signature over canonical `{iss, sub, scope, exp}` JSON |
| `sig_verified` | boolean | no | Populated by the sidecar during verification; ignore on the way in |

### Response (200 OK)

```json
{
  "result": {
    "allow": true,
    "reason": "chain verified; terminates in caller; covers action",
    "reason_code": "chain_ok",
    "obligations": {
      "audit_stream": "sensitive_actions",
      "redact_fields": ["passport_number"]
    }
  }
}
```

`allow` is always present. `reason` is a human-readable string.
`reason_code` is a stable machine-readable identifier that clients
can switch on. `obligations` is a free-form object; keys are
policy-defined.

Well-known reason codes:

| Code | Meaning |
| ---- | ------- |
| `chain_ok` | Chain valid, terminates in caller, covers action |
| `chain_invalid` | Signature failure, expired grant, or hops don't link |
| `chain_wrong_delegate` | Chain doesn't terminate in caller |
| `chain_missing_scope` | Chain doesn't cover the requested action |
| `time_out_of_window` | Business-hours policy denied |
| `tool_scope_ok` | Scoped-tool policy allowed |
| `sensitive_with_obligations` | Allow + obligations returned |
| `policy_undefined` | No policy rule matched (default deny) |
| `policy_shape_error` | Policy returned a value that is not a Decision object |
| `default_deny` | Explicit default-deny hit |

Custom policies may return custom codes. Convention: `snake_case`,
namespaced with policy id if collisions are likely.

### Response (4xx / 5xx)

Client and server errors return a top-level `error` object rather than a
`result`, so callers can distinguish a policy deny from a request-level
failure:

```json
{
  "error": "request body missing top-level `input` object",
  "code": "missing_input",
  "status": 400
}
```

Well-known error codes:

| HTTP | Code | Meaning |
| ---- | ---- | ------- |
| 400 | `json_parse_error` | Body was not valid JSON |
| 400 | `missing_input` | Body missing `input` |
| 413 | `body_too_large` | Body exceeded 1 MiB |
| 415 | `unsupported_media_type` | Content-Type not JSON |
| 405 | `method_not_allowed` | Non-POST hit /authorize |
| 500 | `policy_eval_error` | OPA raised an error (bug in policy) |

## GET /healthz

Liveness probe. Always returns 200 `ok` if the process is running.

## GET /readyz

Readiness probe. Returns 200 `ready` when the policy engine has a
compiled policy set loaded; 503 `not ready` otherwise. Point your
load balancer at this, not `/healthz`.

## Compatibility

### Google Agent2Agent (A2A) spec

The A2A spec's delegation-chain header format is a superset of what this
sidecar consumes. If your traffic already carries an A2A-shaped
`X-A2A-Delegation-Chain` header, a thin adapter can extract it and POST
to `/authorize` unchanged. See
[`examples/planner-executor/executor/main.go`](../examples/planner-executor/executor/main.go)
for the extraction pattern.

### W3C Verifiable Credentials 2.0

v0.1 uses a custom grant shape (see above). VC 2.0 support is on the
v0.3 roadmap. When it lands, the sidecar will accept either shape and
policies will get a normalized view via `input.delegation_chain[]`
regardless of the underlying format.
