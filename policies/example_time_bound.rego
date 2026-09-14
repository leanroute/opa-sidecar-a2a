# package a2a.authorize.time_bound
#
# Reference policy: deny outside business hours regardless of chain
# validity. Illustrates how to layer context-driven rules on top of the
# chain-verification baseline.
#
# "Business hours" here is 09:00–18:00 UTC weekdays. Adjust for your
# team's actual timezone / operating window.

package a2a.authorize.time_bound

import future.keywords.if

default result := {
	"allow": false,
	"reason": "outside business hours",
	"reason_code": "time_out_of_window",
	"obligations": {},
}

result := {
	"allow": true,
	"reason": "within business hours; deferring to chain-based policies",
	"reason_code": "time_ok",
	"obligations": {},
} if {
	in_business_hours
}

in_business_hours if {
	weekday := time.weekday(time.now_ns())
	weekday != "Saturday"
	weekday != "Sunday"
	hour := time.clock(time.now_ns())[0]
	hour >= 9
	hour < 18
}
