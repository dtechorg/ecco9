// wisdomd runs the seven-dimensional wisdom and goal orchestration service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-wisdom/server"
)

func main() {
	svc := server.New()

	addr := os.Getenv("ECCO9_WISDOM_ADDR")
	if addr == "" {
		addr = ":8089"
	}
	log.Printf("ecco9-wisdom listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
