// echobeatsd runs the EchoBeats 12-step cognitive loop service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-echobeats/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_ECHOBEATS_ADDR")
	if addr == "" {
		addr = ":8085"
	}
	log.Printf("ecco9-echobeats listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
