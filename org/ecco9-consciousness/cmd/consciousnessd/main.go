// consciousnessd runs the stream-of-consciousness service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-consciousness/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_CONSCIOUSNESS_ADDR")
	if addr == "" {
		addr = ":8086"
	}
	log.Printf("ecco9-consciousness listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
