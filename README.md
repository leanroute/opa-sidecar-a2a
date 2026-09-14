# opa-sidecar-a2a

**A reference implementation of chain-aware authorization for agent-to-agent (A2A) traffic, using [Open Policy Agent](https://www.openpolicyagent.org/) as the enforcement engine.**

Status: **v0.1.0-alpha** — reference implementation, not a hardened production build. See [Roadmap](#roadmap).

License: MIT.

Maintained by [Leanroute](https://leanroute.dev). Contributions welcome.

---

## Why this exists

Agent-to-agent traffic (one AI agent calling another) is going into production faster than the authorization models are catching up. The industry defaults today are two things, both wrong:

1. **API-key trust.** Agent B trusts agent A because agent A holds a valid API key. This tells B nothing about whether A is authorized to call B *on behalf of the specific user or workflow that originated the request*. A compromised or misbehaving agent can call anywhere its key allows.

2. **Bearer-token pass-through.** Agent A forwards the user's original bearer token to agent B. B checks it and allows. This assumes agent A is trusted enough to hold the user's credentials, and eliminates the ability to distinguish "the user allowed A to do X" from "A decided to do X on the user's behalf without telling them."

Neither model survives contact with a real delegation chain: `user → planner agent → executor agent → tool call → downstream API`. What we need is **chain-aware authorization** — every hop carries verifiable evidence of who authorized what, and the policy engine at the callee side evaluates the whole chain against a policy, not just the immediate caller.

That's the pattern this sidecar implements.

## What "chain-aware authorization" means in practice

Every A2A call carries a **delegation chain** — an ordered list of signed assertions, each of the form "principal P authorizes principal Q to perform action A on resource R, within constraints C, valid until T." The chain grows with every hop.

The callee's sidecar:

1. Verifies every signature in the chain against known trust anchors.
2. Checks that each hop's grant is not expired and covers the action being attempted.
3. Evaluates a Rego policy that encodes the callee's own additional rules (rate limits, sensitive-resource restrictions, per-user allowlists, obligations to attach to the response).
4. Returns `allow` / `deny` with a reason and optional obligations (e.g., "MUST redact PII from response," "MUST log to audit stream X").

The gateway (or the callee itself) can then enforce the decision and enforce any returned obligations.

## Architecture

```
                        ┌──────────────────────────┐
                        │   OPA Sidecar (this)     │
                        │  ─────────────────────   │
                        │  Rego policy engine      │
                        │  Chain verification      │
                        │  Trust anchor management │
                        └────────────▲─────────────┘
                                     │ HTTP POST /authorize
                                     │
   User ──► Planner ──► Executor ──► Tool
              │            │           │
              │            │           └── (also asks sidecar)
              │            └────────────── (also asks sidecar)
              └─────────────────────────── (also asks sidecar)
```

The sidecar is deployed alongside every agent that needs to make authorization decisions on incoming A2A calls. Wire protocol is HTTP+JSON (see [`docs/protocol.md`](docs/protocol.md) for the schema). No SDK required — any language that speaks HTTP can consume it.

**Alternative wire protocols** (Envoy ext_authz gRPC, gRPC-native) are on the roadmap. See [Roadmap](#roadmap).

## Quick start

Two paths depending on whether you have Go installed. Both end in the
same place: sidecar on `localhost:8181`, ready for the worked demo.

### Path A: Docker only (no Go required)

```bash
git clone https://github.com/leanroute/opa-sidecar-a2a
cd opa-sidecar-a2a

# Build image + start sidecar
docker compose up --build -d sidecar

# Run the worked planner-executor demo end-to-end
docker compose --profile demo up --build

# Tear down when done
docker compose --profile demo down
```

### Path B: Local Go install

Prereq: Go 1.22+. On Windows: `winget install GoLang.Go`. On macOS:
`brew install go`. On Linux: your distro's package or
[go.dev/dl](https://go.dev/dl).

**Linux / macOS (has `make`):**

```bash
git clone https://github.com/leanroute/opa-sidecar-a2a
cd opa-sidecar-a2a
go mod tidy
make build
./bin/opa-sidecar --policy-dir ./policies --listen :8181
# in another shell:
cd examples/planner-executor && ./run-demo.sh
# or:
make demo
```

**Windows (PowerShell, no `make` required):**

```powershell
git clone https://github.com/leanroute/opa-sidecar-a2a
cd opa-sidecar-a2a
go mod tidy
.\make.ps1 build
.\bin\opa-sidecar.exe --policy-dir .\policies --listen :8181
# in another PowerShell window:
.\make.ps1 demo
```

`make.ps1` mirrors every Makefile target (`build`, `test`, `run`,
`demo`, `fmt`, `vet`, `tidy`, `clean`). Run `.\make.ps1 help` for the
list.

Either path should show:

```
[planner]   received user request: "book flight LHR → SIN"
[planner]   built delegation chain, calling executor
[executor]  received request from planner
[executor]  asking sidecar: is this call authorized?
[sidecar]   verifying chain: user → planner → executor
[sidecar]   evaluating policy: allow
[executor]  proceeding with tool call
[executor]  tool call complete, returning to planner
```

## Wire protocol

**Request:**
```json
POST /authorize
Content-Type: application/json

{
  "input": {
    "caller": {
      "id": "did:web:planner.example.com",
      "type": "agent"
    },
    "callee": {
      "id": "did:web:executor.example.com",
      "type": "agent"
    },
    "action": "invoke_tool",
    "resource": "flight_booking_v1",
    "delegation_chain": [
      {
        "iss": "did:web:user.example.com",
        "sub": "did:web:planner.example.com",
        "scope": ["book_flight"],
        "exp": 1789430400,
        "sig": "..."
      },
      {
        "iss": "did:web:planner.example.com",
        "sub": "did:web:executor.example.com",
        "scope": ["invoke_tool:flight_booking_v1"],
        "exp": 1789430400,
        "sig": "..."
      }
    ],
    "context": {
      "request_id": "req_abc123",
      "user_agent": "planner/0.1"
    }
  }
}
```

**Response (allow):**
```json
{
  "result": {
    "allow": true,
    "reason": "chain verified; policy allow_delegated_tool_invocation matched",
    "obligations": {
      "audit_stream": "sensitive_actions",
      "redact_fields": ["passport_number"]
    }
  }
}
```

**Response (deny):**
```json
{
  "result": {
    "allow": false,
    "reason": "delegation chain missing scope 'invoke_tool:flight_booking_v1'; planner→executor grant only carries 'invoke_tool:hotel_booking_v1'",
    "obligations": {}
  }
}
```

Full schema, error codes, and edge cases: [`docs/protocol.md`](docs/protocol.md).

## Worked example

The [`examples/planner-executor/`](examples/planner-executor/) folder contains a two-agent demo — a planner that receives a user request, builds a signed delegation chain, and calls an executor that verifies the chain via the sidecar. Runs standalone with `./run-demo.sh`.

For a broader ecosystem view: Google's [Agent2Agent (A2A) SDK samples](https://github.com/google-a2a) show how the delegation shape can look across a wider set of scenarios. This sidecar is compatible with the A2A spec's delegation-chain header format (see [`docs/protocol.md#compatibility`](docs/protocol.md)).

## Policies

Reference policies live in [`policies/`](policies/). They are meant to be copied, adapted, and hardened for your deployment.

| File | What it covers |
| ---- | -------------- |
| `chain_auth.rego` | Core chain-verification helpers: `chain_valid`, `chain_covers_action`, `chain_not_expired` |
| `example_delegation.rego` | Basic pattern: allow if chain is valid and terminates in the caller |
| `example_scoped_tool.rego` | Allow only if the chain grants the specific tool scope being invoked |
| `example_time_bound.rego` | Deny outside business hours regardless of chain validity |
| `example_obligations.rego` | Return obligations (redaction, audit stream) alongside allow |

See [`docs/policy-guide.md`](docs/policy-guide.md) for how to write your own.

## Roadmap

**v0.1.0-alpha (current):**
- HTTP+JSON `/authorize` endpoint
- Embedded OPA engine with hot-reload from a policy directory
- Reference chain verification (ed25519 + JWT signing)
- Worked planner-executor demo
- Reference Rego policies for common patterns

**v0.2.0 (planned):**
- Trust-anchor rotation and revocation
- Envoy ext_authz gRPC protocol support
- OpenTelemetry traces on every authorize decision
- Sidecar-to-sidecar gossip for shared revocation state

**v0.3.0 (proposed):**
- Compatibility with W3C Verifiable Credentials Data Model 2.0 delegation format
- Native support for [MCP Transparency Log](https://github.com/leanroute/mcp-transparency-log) for cross-execution evidence
- Reference integration with [Leanroute](https://leanroute.dev)'s A2A proxy (when that ships)

## Non-goals

- **Not a full IdP.** The sidecar doesn't issue credentials, manage user accounts, or handle OAuth flows. It verifies chains that other systems have already built.
- **Not a service mesh.** No sidecar-to-sidecar traffic interception, no mTLS termination. That's Envoy's job; you can put this sidecar behind Envoy if you want that.
- **Not a policy authoring tool.** Rego is the language. If you want a higher-level DSL, use [Styra Load](https://www.styra.com/) or similar and compile down.

## Contributing

PRs welcome, especially:

- Additional reference policies for common delegation patterns
- Non-Go client examples (Python, TypeScript, Rust)
- Integration examples with popular A2A implementations
- Bug reports and hardening findings

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the workflow.

## Related work

- [Google Agent2Agent (A2A) protocol](https://github.com/google-a2a) — the delegation-chain shape this sidecar consumes
- [Open Policy Agent](https://www.openpolicyagent.org/) — the engine
- [W3C Verifiable Credentials](https://www.w3.org/TR/vc-data-model-2.0/) — the credential-format direction we're tracking for v0.3.0
- [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) — MCP tool calls are a natural place to apply chain-aware auth; see the MCP Transparency Log project (Leanroute, planned) for the auditability side
