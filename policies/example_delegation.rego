# package a2a.authorize
#
# Baseline reference policy: allow if the delegation chain is valid,
# terminates in the caller, and covers the action being attempted.
# Copy + adapt for real deployments. This is the minimum you should
# demand of any A2A call.

package a2a.authorize

import data.a2a.chain
import future.keywords.if

default result := {
	"allow": false,
	"reason": "no matching allow rule",
	"reason_code": "default_deny",
	"obligations": {},
}

# Primary allow rule.
#
# Semantic note: the chain terminates at the CALLEE, not the caller.
# The caller (planner in the demo) is initiating the call; the callee
# (executor in the demo) is the principal the chain grants permission
# TO. Chain shape: user -> planner -> executor. Last grant's sub is
# the callee.
result := {
	"allow": true,
	"reason": "chain verified; terminates in callee; covers action",
	"reason_code": "chain_ok",
	"obligations": {},
} if {
	chain.chain_valid
	chain.chain_terminates_in(input.callee.id)
	chain.chain_covers_action(input.action)
}

# Explicit deny with a specific reason if the chain is bad. Helps ops
# distinguish "no policy matched" from "chain failed verification."
result := {
	"allow": false,
	"reason": "delegation chain failed verification (signatures / expiry / linkage)",
	"reason_code": "chain_invalid",
	"obligations": {},
} if {
	not chain.chain_valid
}

result := {
	"allow": false,
	"reason": sprintf("delegation chain does not terminate in callee %q", [input.callee.id]),
	"reason_code": "chain_wrong_delegate",
	"obligations": {},
} if {
	chain.chain_valid
	not chain.chain_terminates_in(input.callee.id)
}

result := {
	"allow": false,
	"reason": sprintf("delegation chain does not cover action %q", [input.action]),
	"reason_code": "chain_missing_scope",
	"obligations": {},
} if {
	chain.chain_valid
	chain.chain_terminates_in(input.callee.id)
	not chain.chain_covers_action(input.action)
}
