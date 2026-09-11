// emotiond runs the embodied emotion service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-emotion/server"
)

func main() {
	dimensions := 12
	svc := server.New(dimensions)
	defer svc.Close()

	addr := os.Getenv("ECCO9_EMOTION_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	log.Printf("ecco9-emotion listening on %s (aar_dims=%d)", addr, dimensions)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
