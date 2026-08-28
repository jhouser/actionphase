// Command genopenapi renders the OpenAPI document and writes it to a file.
//
// The spec is generated from Go types by huma (paths and schemas) plus the
// document metadata in pkg/docs. This command exists so the result can be
// committed: `just check-api-docs` regenerates it and fails if the committed
// copy differs, which turns API documentation drift into a diff a reviewer can
// read rather than a heuristic comparison of router source against prose.
//
// It builds the router but never listens, and touches no database — rendering
// the spec only reflects over the registered operations' Go types.
//
// Usage:
//
//	go run ./cmd/genopenapi -o pkg/docs/openapi.gen.yaml
package main

import (
	"flag"
	"fmt"
	"os"

	"actionphase/pkg/core"
	apphttp "actionphase/pkg/http"
)

func main() {
	out := flag.String("o", "", "file to write (default: stdout)")
	flag.Parse()

	// A nil pool is deliberate: Router only wires handlers and registers
	// operations. Anything that would dial the database happens per-request,
	// and no request is served here.
	h := &apphttp.Handler{App: core.NewSpecApp()}

	_, docsHandler := h.Router()
	spec, err := docsHandler.Spec()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to render spec: %v\n", err)
		os.Exit(1)
	}

	if *out == "" {
		os.Stdout.Write(spec)
		return
	}

	if err := os.WriteFile(*out, spec, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", *out, err)
		os.Exit(1)
	}
}
