// metacogd runs the metacognitive monitoring service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-metacog/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_METACOG_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	log.Printf("ecco9-metacog listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
