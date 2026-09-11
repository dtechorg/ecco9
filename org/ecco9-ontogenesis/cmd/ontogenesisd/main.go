// ontogenesisd runs the ontogenetic evolution and entelechy service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-ontogenesis/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_ONTOGENESIS_ADDR")
	if addr == "" {
		addr = ":8091"
	}
	log.Printf("ecco9-ontogenesis listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
