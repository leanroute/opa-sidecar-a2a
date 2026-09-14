# Examples

Two paths for exercising the sidecar:

## 1. Worked example: planner → executor (in this repo)

[`planner-executor/`](planner-executor/) contains a minimal two-agent Go
demo that exercises the full chain-aware authorization flow:

- **Planner** receives a user request, builds a signed delegation chain
  (user → planner → executor), calls the executor over HTTP.
- **Executor** receives the request, POSTs the input + delegation chain
  to the sidecar's `/authorize` endpoint, honors the decision.
- **Sidecar** verifies signatures, evaluates the reference policies,
  returns allow / deny + obligations.

Run it:

```bash
cd examples/planner-executor
./run-demo.sh
```

You should see the sidecar allow a well-formed request and deny a
tampered one — three scenarios in total. See
[`planner-executor/README.md`](planner-executor/README.md) for the
full walkthrough.

## 2. Google Agent2Agent SDK samples (external)

The Google A2A SDK ships end-to-end samples that show the delegation-chain
shape in a richer multi-agent context — planner + executor + tool + retry
patterns — using their SDK for the transport layer. If you want to see
what "in production" A2A can look like, that's the reference:

- **A2A SDK samples repository:** https://github.com/google-a2a

This sidecar is compatible with the A2A spec's delegation-chain header
format. See [`docs/protocol.md#compatibility`](../docs/protocol.md) for
the specifics.

The reason both paths matter:

- The **in-repo demo** proves the sidecar works standalone with zero
  external dependencies. Good for verifying your build and understanding
  the shape.
- The **SDK samples** show how the sidecar fits into a realistic A2A
  deployment where the transport, discovery, and retry story is handled
  by a real SDK rather than curl scripts.
