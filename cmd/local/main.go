package main

import (
	"log"
	"net/http"
	"os"

	"tailshell/adbx"
	"tailshell/app"
)

func main() {
	if !adbx.SealSecretConfigured() {
		log.Fatal("set SEAL_SECRET to a random string of at least 16 characters, e.g. SEAL_SECRET=$(openssl rand -hex 32)")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	log.Printf("tailshell listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, http.HandlerFunc(app.Handler)))
}
