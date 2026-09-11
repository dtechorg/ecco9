// atomspaced runs the AtomSpace hypergraph service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-atomspace/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_ATOMSPACE_ADDR")
	if addr == "" {
		addr = ":8087"
	}
	log.Printf("ecco9-atomspace listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
