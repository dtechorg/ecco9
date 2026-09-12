// identityd runs the identity embedding and persona service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-identity/server"
)

func main() {
	stateDir := os.Getenv("ECCO9_IDENTITY_STATE_DIR")
	name := os.Getenv("ECCO9_IDENTITY_NAME")
	svc := server.New(stateDir, name)

	addr := os.Getenv("ECCO9_IDENTITY_ADDR")
	if addr == "" {
		addr = ":8084"
	}
	log.Printf("ecco9-identity listening on %s (identity=%q state=%q)", addr, svc.State.IdentityName, stateDir)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
