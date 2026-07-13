package main

import (
	"log"
	"net/http"
	"os"

	"github.com/kravtandr/keychecker/internal/check"
	"github.com/kravtandr/keychecker/internal/httpapi"
)

func main() {
	token := os.Getenv("KEYCHECKER_TOKEN")
	if token == "" {
		log.Fatal("KEYCHECKER_TOKEN is required (fail-closed): refusing to start without an auth token")
	}
	addr := os.Getenv("KEYCHECKER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	svc := check.NewService(check.DefaultProviders())
	mux := httpapi.NewMux(token, svc)

	log.Printf("keychecker listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
