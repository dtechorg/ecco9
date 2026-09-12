// echodreamd runs the dream-cycle memory consolidation service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-echodream/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_ECHODREAM_ADDR")
	if addr == "" {
		addr = ":8092"
	}
	log.Printf("ecco9-echodream listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
