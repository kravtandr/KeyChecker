package main

import (
	"log"
	"net/http"
	"os"
)

func newMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	addr := os.Getenv("KEYCHECKER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("keychecker listening on %s", addr)
	if err := http.ListenAndServe(addr, newMux()); err != nil {
		log.Fatal(err)
	}
}
