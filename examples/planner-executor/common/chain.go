package common

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Grant is one hop in a delegation chain.
//
// SigVerified is a runtime-populated boolean the sidecar sets after it
// checks Sig against the trust dir. The Rego policies read it as
// `input.delegation_chain[i].sig_verified` (see policies/chain_auth.rego).
// Callers of this helper should NOT set SigVerified; leaving it false
// forces the sidecar to make its own verification pass.
type Grant struct {
	Iss   string   `json:"iss"`
	Sub   string   `json:"sub"`
	Scope []string `json:"scope"`
	Exp   int64    `json:"exp"`
	Sig   string   `json:"sig"`

	SigVerified bool `json:"sig_verified,omitempty"`
}

// Sign builds and signs a single Grant with the issuer's private key. The
// signature covers the canonical JSON representation of iss/sub/scope/exp.
func Sign(issuer Principal, sub string, scope []string, exp time.Time) Grant {
	g := Grant{
		Iss:   issuer.ID,
		Sub:   sub,
		Scope: scope,
		Exp:   exp.Unix(),
	}
	payload := canonicalPayload(g)
	sig := ed25519.Sign(issuer.PrivateKey, payload)
	g.Sig = base64.StdEncoding.EncodeToString(sig)
	return g
}

// Verify checks a grant's signature against the given public key. Used
// by the executor's local sanity check and by tests; the sidecar performs
// the same check independently before evaluating the Rego policy.
func Verify(g Grant, issuerPub ed25519.PublicKey) bool {
	sig, err := base64.StdEncoding.DecodeString(g.Sig)
	if err != nil {
		return false
	}
	return ed25519.Verify(issuerPub, canonicalPayload(g), sig)
}

func canonicalPayload(g Grant) []byte {
	// Canonical shape excludes Sig and SigVerified so signing + verifying
	// see identical bytes regardless of whether the grant is already
	// signed.
	canonical := struct {
		Iss   string   `json:"iss"`
		Sub   string   `json:"sub"`
		Scope []string `json:"scope"`
		Exp   int64    `json:"exp"`
	}{g.Iss, g.Sub, g.Scope, g.Exp}
	b, err := json.Marshal(canonical)
	if err != nil {
		panic(fmt.Sprintf("marshal grant: %v", err))
	}
	return b
}
