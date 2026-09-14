// executor — receives planner calls, asks the sidecar for an authorization
// decision, honors it. Runs an HTTP server on :8081 by default.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/leanroute/opa-sidecar-a2a/examples/planner-executor/common"
)

func main() {
	listen := flag.String("listen", ":8081", "Executor HTTP listen address")
	sidecarURL := flag.String("sidecar-url", "http://localhost:8181/authorize", "Sidecar /authorize endpoint")
	flag.Parse()

	logger := log.New(os.Stdout, "[executor]  ", log.LstdFlags|log.LUTC)

	http.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("received request from planner")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var input map[string]any
		if err := json.Unmarshal(body, &input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Locally pre-verify signatures on the chain. In a production
		// deployment the sidecar does this authoritatively — we do it
		// here purely so demo output shows the failure mode nicely.
		chainRaw, _ := input["delegation_chain"].([]any)
		verifiedChain := make([]map[string]any, 0, len(chainRaw))
		for _, raw := range chainRaw {
			m := raw.(map[string]any)
			g := common.Grant{
				Iss:   m["iss"].(string),
				Sub:   m["sub"].(string),
				Exp:   int64(m["exp"].(float64)),
				Sig:   m["sig"].(string),
				Scope: toStringSlice(m["scope"]),
			}
			pub := common.LookupPublicKey(g.Iss)
			verified := common.Verify(g, pub)
			m["sig_verified"] = verified
			verifiedChain = append(verifiedChain, m)
		}
		input["delegation_chain"] = verifiedChain

		logger.Printf("asking sidecar: is this call authorized?")
		decision, err := askSidecar(*sidecarURL, input)
		if err != nil {
			logger.Printf("sidecar call failed: %v", err)
			http.Error(w, "sidecar error", http.StatusInternalServerError)
			return
		}

		allow, _ := decision["allow"].(bool)
		reasonCode, _ := decision["reason_code"].(string)
		if allow {
			logger.Printf("sidecar decision: ALLOW (reason_code=%s)", reasonCode)
			logger.Printf("proceeding with tool call")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":"tool_call_ok"}`))
			return
		}

		reason, _ := decision["reason"].(string)
		logger.Printf("sidecar decision: DENY (reason_code=%s reason=%q)", reasonCode, reason)
		logger.Printf("refusing tool call")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprintf(w, `{"denied":true,"reason_code":%q}`, reasonCode)
	})

	logger.Printf("listening on %s (sidecar at %s)", *listen, *sidecarURL)
	if err := http.ListenAndServe(*listen, nil); err != nil {
		log.Fatal(err)
	}
}

func askSidecar(url string, input map[string]any) (map[string]any, error) {
	payload, _ := json.Marshal(map[string]any{"input": input})
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("sidecar returned status=%d body=%s: %w", resp.StatusCode, string(body), err)
	}
	return env.Result, nil
}

func toStringSlice(v any) []string {
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
