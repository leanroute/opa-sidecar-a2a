# package a2a.authorize.scoped_tool
#
# Reference policy for tool-invocation calls where the chain must grant a
# scope of the shape `invoke_tool:<tool_id>` matching the tool actually
# being invoked. Prevents the "planner had permission to book a flight, so
# it should be able to also invoke the SEND_EMAIL tool" scope-creep bug.
#
# Load this alongside example_delegation.rego. Whichever returns allow first
# wins; both must return an object matching the Decision shape.

package a2a.authorize.scoped_tool

import data.a2a.chain
import future.keywords.if

default result := {
	"allow": false,
	"reason": "no matching allow rule for scoped_tool policy",
	"reason_code": "default_deny",
	"obligations": {},
}

result := {
	"allow": true,
	"reason": sprintf("chain grants specific tool scope %q", [required_scope]),
	"reason_code": "tool_scope_ok",
	"obligations": {},
} if {
	input.action == "invoke_tool"
	chain.chain_valid
	chain.chain_terminates_in(input.callee.id)
	chain.chain_covers_action(required_scope)
}

# Required scope is the tool id, prefixed. `input.resource` carries the
# tool id (see docs/protocol.md).
required_scope := sprintf("invoke_tool:%s", [input.resource])
