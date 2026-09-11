// relevanced runs the Relevance Realization Ennead service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-relevance/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_RELEVANCE_ADDR")
	if addr == "" {
		addr = ":8088"
	}
	log.Printf("ecco9-relevance listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
