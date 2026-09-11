// modelsd runs the model registry and conversion service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-models/server"
)

func main() {
	root := os.Getenv("ECCO9_MODEL_STORE")
	if root == "" {
		root = "/var/lib/ecco9/models"
	}
	svc, err := server.New(root)
	if err != nil {
		log.Fatalf("ecco9-models: %v", err)
	}
	addr := os.Getenv("ECCO9_MODELS_ADDR")
	if addr == "" {
		addr = ":8095"
	}
	log.Printf("ecco9-models listening on %s (store: %s)", addr, root)
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
