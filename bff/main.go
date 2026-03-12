package main

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"bff/auth"
	"bff/proxy"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(jsonHandler))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		slog.Info("Starting Proxy server on port 443...")

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Health check", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
		})
		http.Handle("/api/", proxy.NewHandler("api:8081"))
		http.HandleFunc("/auth/login", auth.LoginHandler)
		http.HandleFunc("/auth/callback", auth.CallbackHandler)

		http.ListenAndServeTLS(":443", "./_keys/server.crt", "./_keys/server.key", nil)
		wg.Done()
	}()

	wg.Wait()
}
