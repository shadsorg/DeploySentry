// Package main is a trivial HTTP server for the canary demo.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	appVersion := os.Getenv("APP_VERSION")
	if appVersion == "" {
		appVersion = "v1.0.0-unknown"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintf(w, "version=%s\n", appVersion)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "ok\n")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Demo app %s listening on :%s", appVersion, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
