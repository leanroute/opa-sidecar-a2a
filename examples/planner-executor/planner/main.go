// planner — receives a user request, builds a signed delegation chain,
// calls the executor. Demo-only; the "user" here is a hardcoded principal
// with a hardcoded private key. See ../common/keys.go.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/leanroute/opa-sidecar-a2a/examples/planner-executor/common"
)

func main() {
	executorURL := flag.String("executor-url", "http://localhost:8081/invoke", "Executor HTTP endpoint")
	scenario := flag.String("scenario", "happy", "One of: happy, scope_creep, tampered")
	flag.Parse()

	logger := log.New(nil, "[planner]   ", log.LstdFlags|log.LUTC)
	logger.SetOutput(newSectionWriter("planner"))

	user := common.User()
	planner := common.Planner()
	executor := common.Executor()

	exp := time.Now().Add(1 * time.Hour)

	var chain []common.Grant
	var action, resource string

	switch *scenario {
	case "happy":
		logger.Printf("received user request: \"book flight LHR → SIN\"")
		chain = []common.Grant{
			common.Sign(user, planner.ID, []string{"book_flight"}, exp),
			common.Sign(planner, executor.ID, []string{"invoke_tool:flight_booking_v1"}, exp),
		}
		action = "invoke_tool"
		resource = "flight_booking_v1"

	case "scope_creep":
		logger.Printf("attempting invoke_tool on send_email_v1 (never authorized by user)")
		chain = []common.Grant{
			common.Sign(user, planner.ID, []string{"book_flight"}, exp),
			common.Sign(planner, executor.ID, []string{"invoke_tool:flight_booking_v1"}, exp),
		}
		// action + resource intentionally do NOT match anything in the chain.
		action = "invoke_tool"
		resource = "send_email_v1"

	case "tampered":
		logger.Printf("building tampered chain (attacker widens scope after signing)")
		// Legitimate chain first.
		chain = []common.Grant{
			common.Sign(user, planner.ID, []string{"book_flight"}, exp),
			common.Sign(planner, executor.ID, []string{"invoke_tool:flight_booking_v1"}, exp),
		}
		// Attacker widens the executor's scope. Signature no longer matches.
		chain[1].Scope = []string{"invoke_tool:*"}
		action = "invoke_tool"
		resource = "flight_booking_v1"

	default:
		log.Fatalf("unknown scenario %q; use happy | scope_creep | tampered", *scenario)
	}

	logger.Printf("built delegation chain user → planner → executor")
	logger.Printf("calling executor with delegation chain")

	body := map[string]any{
		"caller":           map[string]string{"id": planner.ID, "type": "agent"},
		"callee":           map[string]string{"id": executor.ID, "type": "agent"},
		"action":           action,
		"resource":         resource,
		"delegation_chain": chain,
	}
	payload, _ := json.Marshal(body)

	resp, err := http.Post(*executorURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Fatalf("call executor: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	fmt.Printf("[planner]   executor replied status=%d body=%s\n", resp.StatusCode, string(out))
}

// sectionWriter is a tiny io.Writer that prefixes every line with a
// scenario marker so the demo output is readable.
type sectionWriter struct{ prefix string }

func newSectionWriter(prefix string) *sectionWriter { return &sectionWriter{prefix: prefix} }

func (s *sectionWriter) Write(p []byte) (int, error) {
	return fmt.Fprintf(fmtWriter{}, "%s", string(p))
}

type fmtWriter struct{}

func (fmtWriter) Write(p []byte) (int, error) { return fmt.Print(string(p)) }
