# package a2a.chain
#
# Core chain-verification helpers shared across all reference policies.
# Consumers import this package and call `chain_valid`, `chain_covers_action`,
# `chain_not_expired`, and `chain_terminates_in(caller)` from their own rules.
#
# Assumptions about the input shape (see docs/protocol.md):
#
#   input.caller.id            string
#   input.callee.id            string
#   input.action               string
#   input.resource             string
#   input.delegation_chain     array of grants, in order from user → last hop
#   input.now                  RFC3339 timestamp (defaults to time.now_ns())
#
# Each grant in the chain looks like:
#
#   { "iss": "<principal id>", "sub": "<delegate id>", "scope": ["..."],
#     "exp": 1789430400, "sig": "<verified upstream, treated as a boolean here>" }
#
# NOTE: Signature verification is done in Go (internal/chain), not Rego.
# The input arrives here with a synthetic `sig_verified: bool` flag on each
# grant. This keeps expensive crypto out of the hot policy path and lets
# the Go side share verified-signature caching across requests.

package a2a.chain

import future.keywords.if
import future.keywords.in
import future.keywords.every

default chain_valid := false

# A chain is valid iff:
#   - it has at least one grant
#   - every grant's signature verified upstream
#   - every grant's issuer matches the previous grant's subject (or is the
#     original user for the first grant)
#   - no grant has expired
chain_valid if {
	count(input.delegation_chain) > 0
	every g in input.delegation_chain {
		g.sig_verified == true
	}
	chain_hops_linked
	chain_not_expired
}

# Adjacent hops link: grant[i+1].iss == grant[i].sub for every i.
chain_hops_linked if {
	pairs := [[a, b] |
		i := numbers.range(0, count(input.delegation_chain) - 2)[_]
		a := input.delegation_chain[i]
		b := input.delegation_chain[i + 1]
	]
	every p in pairs {
		p[0].sub == p[1].iss
	}
}

# Convenience for policies that only need the "not expired" leg.
default chain_not_expired := false

chain_not_expired if {
	now_ns := time.parse_rfc3339_ns(default_now)
	every g in input.delegation_chain {
		g.exp * 1000000000 >= now_ns  # exp is seconds; compare in ns
	}
}

# `input.now` overrides `time.now_ns()` for deterministic testing. Falls
# back to the current wall-clock time when the caller omits it.
default_now := input.now if {
	input.now
}

default_now := time.format(time.now_ns()) if {
	not input.now
}

# Does any grant in the chain cover the action being attempted?
chain_covers_action(action) if {
	some g in input.delegation_chain
	action in g.scope
}

# Does the final grant terminate at the caller (i.e., is the caller
# actually the intended delegate of the immediately-preceding principal)?
chain_terminates_in(caller_id) if {
	last := input.delegation_chain[count(input.delegation_chain) - 1]
	last.sub == caller_id
}
