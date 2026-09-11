// reservoird runs the Echo State Reservoir service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-reservoir/reservoir"
	"github.com/dtechorg/ecco9-reservoir/server"
)

func main() {
	size := 100
	outDim := 16
	svc := server.New(size, reservoir.PersonaContemplativeScholar, outDim)

	addr := os.Getenv("ECCO9_RESERVOIR_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	log.Printf("ecco9-reservoir listening on %s (size=%d out=%d)", addr, size, outDim)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
