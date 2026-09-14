// Package main is the entry point for the opa-sidecar-a2a binary.
//
// The sidecar exposes a single HTTP endpoint (POST /authorize) that agents
// or gateways call to make chain-aware authorization decisions on A2A
// traffic. Policy is expressed as Rego and loaded from a directory at
// startup, with hot-reload on SIGHUP.
//
// Design principles:
//
//   - Zero-configuration first run. `opa-sidecar --policy-dir ./policies`
//     should Just Work, so the reference implementation is easy to try.
//   - No hidden state. Every decision is a pure function of (request,
//     loaded policies, trust anchors). No database, no cache mutation
//     inside a request handler.
//   - Fail closed. If policies fail to load, if signatures don't verify,
//     if the input is malformed, the sidecar returns deny with a reason.
//     Never allow-by-default.
//   - Machine-readable reasons. Every deny carries a `reason` string that
//     a human can read AND a `reason_code` a machine can switch on.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leanroute/opa-sidecar-a2a/internal/policy"
	"github.com/leanroute/opa-sidecar-a2a/internal/server"
)

// Build-time-injected version string. `go build -ldflags "-X main.version=v0.1.0"`.
var version = "dev"

func main() {
	var (
		listenAddr   = flag.String("listen", ":8181", "HTTP listen address")
		policyDir    = flag.String("policy-dir", "./policies", "Directory of .rego policy files")
		trustDir     = flag.String("trust-dir", "./trust", "Directory of trusted signer public keys (PEM)")
		reloadOnHup  = flag.Bool("reload-on-sighup", true, "Hot-reload policies + trust anchors on SIGHUP")
		showVer      = flag.Bool("version", false, "Print version and exit")
		readTimeout  = flag.Duration("read-timeout", 5*time.Second, "HTTP server read timeout")
		writeTimeout = flag.Duration("write-timeout", 5*time.Second, "HTTP server write timeout")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("opa-sidecar-a2a", version)
		return
	}

	logger := log.New(os.Stderr, "[opa-sidecar] ", log.LstdFlags|log.LUTC)
	logger.Printf("starting version=%s listen=%s policy-dir=%s", version, *listenAddr, *policyDir)

	// Load policy engine. Fail-fast on startup: if we cannot compile the
	// policies we don't want to serve any traffic returning "internal
	// error" — we want to loudly refuse to start.
	engine, err := policy.NewEngine(*policyDir, *trustDir, logger)
	if err != nil {
		logger.Fatalf("failed to initialize policy engine: %v", err)
	}

	handler := server.NewHandler(engine, logger)

	mux := http.NewServeMux()
	mux.Handle("/authorize", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if engine.Ready() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	})

	srv := &http.Server{
		Addr:         *listenAddr,
		Handler:      withRequestLogging(logger, mux),
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}

	// SIGHUP → hot-reload policies. Keeps the same server up so in-flight
	// requests aren't dropped.
	if *reloadOnHup {
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		go func() {
			for range hup {
				logger.Printf("SIGHUP received, reloading policies + trust anchors")
				if err := engine.Reload(); err != nil {
					logger.Printf("reload failed (keeping previous policy set): %v", err)
					continue
				}
				logger.Printf("reload complete")
			}
		}()
	}

	// Graceful shutdown on SIGTERM / SIGINT.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		logger.Printf("shutdown signal received, draining...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	logger.Printf("ready on %s", *listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
	logger.Printf("shutdown complete")
}

// withRequestLogging wraps a handler with a minimal request log line
// (method, path, status, duration). Deliberately does NOT log the input
// body — delegation chains carry signed claims that shouldn't land in
// stdout by accident.
func withRequestLogging(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Printf("%s %s status=%d dur=%s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
