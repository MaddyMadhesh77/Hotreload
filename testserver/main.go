// testserver is a minimal HTTP server used to demonstrate the hotreload tool.
// Edit the VERSION constant, save the file, and watch hotreload rebuild and
// restart the server automatically.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// VERSION is intentionally a plain constant so engineers can edit it to
// trigger a visible hot-reload: change the string, save, and the server
// will restart with the new value within seconds.
const VERSION = "v1.0.0"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request received", "method", r.Method, "path", r.URL.Path)
		fmt.Fprintf(w, "testserver %s — %s\n", VERSION, time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":%q}`, VERSION)
	})

	addr := ":" + port
	slog.Info("testserver starting", "addr", addr, "version", VERSION)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("testserver: %v", err)
	}
}
