# package a2a.authorize.obligations
#
# Reference policy: return obligations alongside an allow decision.
# Obligations are instructions the caller MUST honor to remain in policy
# compliance — e.g., "redact these fields from the response before
# forwarding," or "emit an audit event to this stream."
#
# This is the escape hatch for policies that say "allow, but only if
# the caller does X." Common uses: PII redaction, mandatory audit
# logging, response-size caps, obligation to attach a specific claim
# to the downstream call.

package a2a.authorize.obligations

import data.a2a.chain
import future.keywords.if

default result := {
	"allow": false,
	"reason": "obligations policy default deny",
	"reason_code": "default_deny",
	"obligations": {},
}

result := {
	"allow": true,
	"reason": "chain ok; sensitive resource requires redaction + audit",
	"reason_code": "sensitive_with_obligations",
	"obligations": {
		"redact_fields": ["passport_number", "date_of_birth", "credit_card"],
		"audit_stream": "sensitive_actions",
		"max_response_bytes": 65536,
	},
} if {
	chain.chain_valid
	chain.chain_terminates_in(input.callee.id)
	is_sensitive_resource
}

is_sensitive_resource if {
	input.resource == "flight_booking_v1"
}

is_sensitive_resource if {
	input.resource == "identity_lookup_v1"
}

is_sensitive_resource if {
	input.resource == "payment_v1"
}
