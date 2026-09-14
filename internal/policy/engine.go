// Package policy wraps the embedded OPA engine with a file-directory-based
// policy loader and a Reload() method for SIGHUP hot-reloads.
//
// The engine is deliberately thin: it does not manage caches, revocation
// state, or trust anchors beyond loading them from disk. Those are the
// responsibility of the caller (server.Handler) or a future v0.2 revocation
// module. This keeps the code path for an /authorize request pure.
package policy

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/rego"
)

// Engine holds the compiled Rego policy set and the trust-anchor material
// needed to verify delegation-chain signatures.
type Engine struct {
	policyDir string
	trustDir  string
	logger    *log.Logger

	mu           sync.RWMutex
	preparedEval rego.PreparedEvalQuery // compiled query for `data.a2a.authorize.result`
	trustAnchors map[string][]byte      // key id → PEM-encoded public key
	ready        bool
}

// NewEngine loads policies + trust anchors from disk and compiles them.
// Fails fast if either fails — the sidecar should not start in an
// undefined state.
func NewEngine(policyDir, trustDir string, logger *log.Logger) (*Engine, error) {
	e := &Engine{
		policyDir: policyDir,
		trustDir:  trustDir,
		logger:    logger,
	}
	if err := e.Reload(); err != nil {
		return nil, err
	}
	return e, nil
}

// Reload re-reads the policy dir + trust dir, recompiles, and swaps the
// live state atomically. On failure the existing state is preserved.
func (e *Engine) Reload() error {
	policies, err := readRegoFiles(e.policyDir)
	if err != nil {
		return fmt.Errorf("read policy dir %q: %w", e.policyDir, err)
	}
	if len(policies) == 0 {
		return fmt.Errorf("no .rego files found in %q — refusing to start with an empty policy set", e.policyDir)
	}

	trust, err := readTrustAnchors(e.trustDir)
	if err != nil {
		// Missing trust dir is OK for the reference / demo case — chain
		// verification will simply reject anything that requires a
		// verified signature. We log at INFO so operators notice.
		e.logger.Printf("trust dir %q not usable (%v) — running with empty trust anchor set", e.trustDir, err)
		trust = map[string][]byte{}
	}

	opts := []func(*rego.Rego){
		rego.Query("data.a2a.authorize.result"),
	}
	for path, src := range policies {
		opts = append(opts, rego.Module(path, src))
	}

	prepared, err := rego.New(opts...).PrepareForEval(context.Background())
	if err != nil {
		return fmt.Errorf("compile Rego policies: %w", err)
	}

	e.mu.Lock()
	e.preparedEval = prepared
	e.trustAnchors = trust
	e.ready = true
	e.mu.Unlock()

	e.logger.Printf("loaded %d policy file(s), %d trust anchor(s)", len(policies), len(trust))
	return nil
}

// Ready reports whether the engine has a successfully-compiled policy set
// loaded. Used by /readyz.
func (e *Engine) Ready() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ready
}

// Decision is the shape returned by every /authorize evaluation.
// Mirrors the JSON contract in docs/protocol.md.
type Decision struct {
	Allow       bool                   `json:"allow"`
	Reason      string                 `json:"reason"`
	ReasonCode  string                 `json:"reason_code,omitempty"`
	Obligations map[string]interface{} `json:"obligations,omitempty"`
}

// Evaluate runs the compiled query against the caller-supplied input.
// The input shape matches the "input" object documented in docs/protocol.md.
func (e *Engine) Evaluate(ctx context.Context, input map[string]interface{}) (Decision, error) {
	e.mu.RLock()
	prepared := e.preparedEval
	e.mu.RUnlock()

	rs, err := prepared.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return Decision{}, fmt.Errorf("rego eval: %w", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		// Policy returned undefined — fail closed with a machine-readable
		// reason so callers can distinguish this from an explicit deny.
		return Decision{
			Allow:      false,
			Reason:     "policy returned no decision (undefined)",
			ReasonCode: "policy_undefined",
		}, nil
	}

	raw, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return Decision{
			Allow:      false,
			Reason:     fmt.Sprintf("policy returned unexpected shape %T; expected object", rs[0].Expressions[0].Value),
			ReasonCode: "policy_shape_error",
		}, nil
	}

	return decisionFromMap(raw), nil
}

// TrustAnchor returns the PEM-encoded public key for a given key id, or
// nil if not found. Used by internal/chain during signature verification.
func (e *Engine) TrustAnchor(kid string) []byte {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.trustAnchors[kid]
}

// ─── helpers ────────────────────────────────────────────────────────────

func decisionFromMap(m map[string]interface{}) Decision {
	d := Decision{}
	if v, ok := m["allow"].(bool); ok {
		d.Allow = v
	}
	if v, ok := m["reason"].(string); ok {
		d.Reason = v
	}
	if v, ok := m["reason_code"].(string); ok {
		d.ReasonCode = v
	}
	if v, ok := m["obligations"].(map[string]interface{}); ok {
		d.Obligations = v
	}
	if d.Reason == "" {
		if d.Allow {
			d.Reason = "policy allow (no explicit reason)"
		} else {
			d.Reason = "policy deny (no explicit reason)"
		}
	}
	return d
}

func readRegoFiles(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".rego") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = string(b)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Deterministic iteration for tests + logs.
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := make(map[string]string, len(out))
	for _, k := range keys {
		sorted[k] = out[k]
	}
	return sorted, nil
}

func readTrustAnchors(dir string) (map[string][]byte, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}
	out := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".pem") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		// Convention: filename without .pem is the kid.
		kid := strings.TrimSuffix(name, filepath.Ext(name))
		out[kid] = b
	}
	return out, nil
}
