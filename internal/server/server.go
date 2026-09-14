// Package server implements the HTTP handler that terminates the /authorize
// wire protocol and forwards to the policy engine.
//
// Wire contract: docs/protocol.md.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/leanroute/opa-sidecar-a2a/internal/policy"
)

// Handler wraps a policy.Engine as an http.Handler that speaks the
// /authorize wire protocol.
type Handler struct {
	engine *policy.Engine
	logger *log.Logger
}

// NewHandler constructs an /authorize handler backed by the given engine.
func NewHandler(engine *policy.Engine, logger *log.Logger) *Handler {
	return &Handler{engine: engine, logger: logger}
}

// Request is the JSON envelope callers POST to /authorize. The `input`
// field mirrors OPA's convention — we forward it into the Rego engine
// verbatim, which means policies can bind to `input.caller`,
// `input.delegation_chain`, etc.
type Request struct {
	Input map[string]interface{} `json:"input"`
}

// Response is the JSON envelope we return.
type Response struct {
	Result policy.Decision `json:"result"`
}

// ErrorResponse is returned on 4xx/5xx paths so clients can distinguish a
// policy deny (200 with allow=false) from a request-level failure.
type ErrorResponse struct {
	Error   string `json:"error"`
	Detail  string `json:"detail,omitempty"`
	Code    string `json:"code"`
	Status  int    `json:"status"`
}

// Body size cap. A delegation chain of ~20 hops with modest metadata is
// comfortably under 100KB; anything much bigger is either an attack or a
// caller bug. Fail loud rather than accept it silently.
const maxBodyBytes = 1 << 20 // 1 MiB

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required", "")
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Content-Type must be application/json", "")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large",
				fmt.Sprintf("request body exceeds %d bytes", maxBodyBytes), err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "body_read_error",
			"failed to read request body", err.Error())
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "json_parse_error",
			"request body is not valid JSON", err.Error())
		return
	}
	if req.Input == nil {
		writeError(w, http.StatusBadRequest, "missing_input",
			"request body missing top-level `input` object", "")
		return
	}

	decision, err := h.engine.Evaluate(r.Context(), req.Input)
	if err != nil {
		h.logger.Printf("policy evaluation failed: %v", err)
		writeError(w, http.StatusInternalServerError, "policy_eval_error",
			"policy engine failed to evaluate request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{Result: decision})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg, detail string) {
	writeJSON(w, status, ErrorResponse{
		Error:  msg,
		Detail: detail,
		Code:   code,
		Status: status,
	})
}
