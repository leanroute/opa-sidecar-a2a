# planner-executor

A minimal two-agent worked example for `opa-sidecar-a2a`.

```
┌────────┐      ┌─────────────┐      ┌──────────────┐      ┌────────┐
│  user  │─────►│   planner   │─────►│   executor   │─────►│  tool  │
└────────┘      └─────────────┘      └──────────────┘      └────────┘
                                            │
                                            │ POST /authorize
                                            ▼
                                    ┌──────────────┐
                                    │   sidecar    │
                                    │  (:8181)     │
                                    └──────────────┘
```

## What it demonstrates

Three scenarios that exercise the reference `example_delegation.rego`
and `example_scoped_tool.rego` policies:

1. **Happy path.** User grants planner `book_flight` scope; planner
   grants executor `invoke_tool:flight_booking_v1`; executor asks
   sidecar; sidecar returns `allow`.
2. **Scope-creep attack.** Planner tries to invoke a tool
   (`send_email_v1`) that the user never authorized. The chain is
   signature-valid but does not cover the action. Sidecar returns
   `deny` with reason_code `chain_missing_scope`.
3. **Tampered chain.** Attacker modifies the executor's grant to widen
   the scope. Signature verification fails at the sidecar. Returns
   `deny` with reason_code `chain_invalid`.

## Running

Prerequisite: the sidecar is running on `:8181` with the reference
policies loaded. From the repo root:

```bash
make build
./bin/opa-sidecar --policy-dir ./policies --listen :8181 &
```

Then from this directory:

```bash
./run-demo.sh
```

Alternatively, from the repo root: `make demo` runs both the sidecar
and this demo in sequence.

## Expected output

```
[planner]   received user request: "book flight LHR → SIN"
[planner]   built delegation chain user → planner → executor
[planner]   calling executor with delegation chain

[executor]  received request from planner
[executor]  asking sidecar: is this call authorized?
[sidecar]   POST /authorize dur=2ms status=200
[executor]  sidecar decision: ALLOW (reason_code=chain_ok)
[executor]  proceeding with tool call

── SCENARIO 2: scope creep ──
[planner]   attempting invoke_tool on send_email_v1 (never authorized)
[executor]  sidecar decision: DENY (reason_code=chain_missing_scope)
[executor]  refusing tool call

── SCENARIO 3: tampered chain ──
[planner]   forwarding tampered chain to executor
[executor]  sidecar decision: DENY (reason_code=chain_invalid)
[executor]  refusing tool call
```

## Files

| File | Purpose |
| ---- | ------- |
| `planner/main.go`  | Builds delegation chains, calls executor over HTTP. |
| `executor/main.go` | Receives planner calls, asks sidecar, honors decision. |
| `common/keys.go`   | Test-only ed25519 keypairs for user + planner + executor. |
| `common/chain.go`  | Chain-building helpers shared between agents. |
| `run-demo.sh`      | Boots planner + executor, runs the three scenarios. |
| `trust/`           | Public keys the sidecar's trust dir should hold. |
