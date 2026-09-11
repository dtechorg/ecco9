// gatewayd runs the external-facing ecco9 API gateway.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-gateway/gateway"
)

func main() {
	addr := os.Getenv("ECCO9_GATEWAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	g := gateway.New()
	log.Printf("ecco9-gateway listening on %s (%d backends configured)", addr, len(g.Backends()))
	log.Fatal(http.ListenAndServe(addr, g.Handler()))
}
