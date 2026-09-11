// memoryd runs the hypergraph memory service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-memory/server"
)

func main() {
	stateDir := os.Getenv("ECCO9_MEMORY_STATE_DIR")
	svc := server.New(stateDir)

	addr := os.Getenv("ECCO9_MEMORY_ADDR")
	if addr == "" {
		addr = ":8082"
	}
	log.Printf("ecco9-memory listening on %s (state=%q)", addr, stateDir)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
