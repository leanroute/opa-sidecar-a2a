# Writing your own policies

Rego crash course, tailored to what this sidecar expects.

## The one rule the sidecar cares about

The sidecar prepares a single query:

```
data.a2a.authorize.result
```

Every request evaluates that path. So every policy file that
contributes to the decision must live in package `a2a.authorize` (or a
subpackage thereof) and expose a `result` rule that evaluates to an
object matching the Decision shape:

```rego
result := {
    "allow": true | false,
    "reason": "human-readable string",
    "reason_code": "machine_readable_string",  # optional
    "obligations": { ... }                     # optional
}
```

## Boilerplate for a new policy file

```rego
package a2a.authorize

import data.a2a.chain

# Deny-by-default. Always the first rule.
default result := {
    "allow": false,
    "reason": "no matching allow rule in my_policy.rego",
    "reason_code": "default_deny",
    "obligations": {},
}

# One or more allow rules.
result := {
    "allow": true,
    "reason": "my custom condition matched",
    "reason_code": "my_policy_ok",
    "obligations": {},
} if {
    # your condition here
    chain.chain_valid
    input.action == "my_specific_action"
}
```

Save to `policies/my_policy.rego`, SIGHUP the sidecar, done.

## Available helpers

From `data.a2a.chain` (see `policies/chain_auth.rego`):

| Helper | Returns |
| ------ | ------- |
| `chain.chain_valid` | bool — chain signatures verified, hops linked, not expired |
| `chain.chain_not_expired` | bool — no grant past its `exp` |
| `chain.chain_covers_action(action)` | bool — some grant lists `action` in its scope |
| `chain.chain_terminates_in(id)` | bool — final grant's `sub` == `id` |

## Layering multiple policy files

Multiple `.rego` files can contribute to the same package. OPA merges
rules from all of them at compile time. If two files disagree (one
returns allow, one returns deny) OPA picks whichever rule matched — so
you MUST use `default result := ...` in every file that returns a
Decision, and you MUST avoid overlapping conditions.

Recommended structure for real deployments:

- `policies/00_chain.rego` → the chain helpers (copy from
  `chain_auth.rego` and adjust)
- `policies/10_defaults.rego` → your default-deny rule
- `policies/20_<domain>.rego` → domain-specific allow rules (one file
  per policy domain: `20_tools.rego`, `20_reads.rego`, `20_admin.rego`)
- `policies/90_obligations.rego` → obligations layered on top of allow
  decisions

Numeric prefixes are convention, not enforced. They make the load order
predictable when reading logs.

## Testing policies

The included CI workflow runs `opa test policies/ tests/rego/` if a
`tests/rego/` directory exists. Add fixtures like:

```rego
# tests/rego/example_delegation_test.rego
package a2a.authorize_test

import data.a2a.authorize

test_happy_path_allowed if {
    result := authorize.result with input as {
        "caller":   {"id": "did:web:planner.example.com"},
        "callee":   {"id": "did:web:executor.example.com"},
        "action":   "invoke_tool",
        "resource": "flight_booking_v1",
        "delegation_chain": [
            {"iss": "did:web:user.example.com",
             "sub": "did:web:planner.example.com",
             "scope": ["book_flight"],
             "exp": 9999999999,
             "sig_verified": true},
            {"iss": "did:web:planner.example.com",
             "sub": "did:web:executor.example.com",
             "scope": ["invoke_tool:flight_booking_v1"],
             "exp": 9999999999,
             "sig_verified": true},
        ],
    }
    result.allow == true
}
```

Run locally: `opa test policies/ tests/rego/`.

## Common patterns

**"Allow only during a maintenance window."** See
`policies/example_time_bound.rego` for the shape. Layer on top of
`example_delegation.rego` by putting both in package
`a2a.authorize` and using different subpackage `.time_bound` — the
sidecar evaluates `data.a2a.authorize.result` so only rules in the
top-level `a2a.authorize` package contribute unless you explicitly
merge them.

**"Require a specific scope for a specific tool."** See
`policies/example_scoped_tool.rego`. Uses `input.action == "invoke_tool"`
+ `input.resource` to derive the required scope name.

**"Return obligations."** See `policies/example_obligations.rego`.
Populate the `obligations` field on the returned Decision; the caller
is responsible for honoring them.

## Anti-patterns

- **Don't put crypto in Rego.** Signature verification runs in Go before
  the policy sees the input. Trying to `crypto.hmac.equal` inside Rego
  will be slow and hard to audit.
- **Don't consult external services from Rego.** OPA does support
  `http.send` but the sidecar deliberately does not enable it — every
  decision must be a pure function of the input + loaded state.
- **Don't rely on rule order for correctness.** OPA is declarative;
  reorder-safe. If you find yourself thinking "this rule needs to run
  before that one," rewrite to disjoint conditions.
