package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/kravtandr/keychecker/internal/check"
	"github.com/kravtandr/keychecker/internal/httpapi"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz on the local server and exit 0/1 (for container healthchecks)")
	flag.Parse()

	addr := os.Getenv("KEYCHECKER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	if *healthcheck {
		os.Exit(runHealthcheck(addr))
	}

	token := os.Getenv("KEYCHECKER_TOKEN")
	if token == "" {
		log.Fatal("KEYCHECKER_TOKEN is required (fail-closed): refusing to start without an auth token")
	}

	svc := check.NewService(check.DefaultProviders())
	mux := httpapi.NewMux(token, svc)

	log.Printf("keychecker listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// runHealthcheck пробует /healthz на локальном сервере. Возвращает 0 при 200,
// иначе 1. Используется как self-contained healthcheck в distroless-образе,
// где нет curl/wget.
func runHealthcheck(addr string) int {
	url := "http://127.0.0.1" + addr + "/healthz"
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("healthcheck failed: %v", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck got status %d", resp.StatusCode)
		return 1
	}
	return 0
}
