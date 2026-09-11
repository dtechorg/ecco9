// orchestratord runs the central orchestration service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-orchestrator/server"
)

func main() {
	svc := server.New()
	addr := os.Getenv("ECCO9_ORCHESTRATOR_ADDR")
	if addr == "" {
		addr = ":8096"
	}
	log.Printf("ecco9-orchestrator listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
