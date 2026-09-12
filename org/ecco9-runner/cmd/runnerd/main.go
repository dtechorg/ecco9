// runnerd runs the local GGUF inference runner control plane.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-runner/server"
)

func main() {
	svc := server.New()
	addr := os.Getenv("ECCO9_RUNNER_ADDR")
	if addr == "" {
		addr = ":8094"
	}
	log.Printf("ecco9-runner listening on %s (backend: stub)", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
