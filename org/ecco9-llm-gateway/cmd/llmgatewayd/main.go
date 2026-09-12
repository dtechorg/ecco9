// llmgatewayd runs the multi-provider LLM gateway service.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dtechorg/ecco9-llm-gateway/server"
)

func main() {
	svc, err := server.New()
	if err != nil {
		log.Fatalf("ecco9-llm-gateway: %v", err)
	}
	addr := os.Getenv("ECCO9_LLM_GATEWAY_ADDR")
	if addr == "" {
		addr = ":8093"
	}
	log.Printf("ecco9-llm-gateway listening on %s (current provider: %s)", addr, svc.Gateway.CurrentProvider())
	log.Fatal(http.ListenAndServe(addr, svc.Routes()))
}
