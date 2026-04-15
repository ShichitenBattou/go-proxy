package main

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"bff/auth"
	"bff/proxy"
	"bff/setup"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(jsonHandler))

	cfg := setup.GetConfig()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		slog.Info("Starting BFF server", "addr", cfg.BFFListenAddr)

		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Health check", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
		})
		http.Handle("/", proxy.NewHandler(cfg.ProxyTarget))
		http.HandleFunc("/auth/login", auth.LoginHandler)
		http.HandleFunc("/auth/callback", auth.CallbackHandler)
		http.HandleFunc("/auth/me", auth.MeHandler)
		http.HandleFunc("/auth/logout", auth.LogoutHandler)

		http.ListenAndServe(cfg.BFFListenAddr, nil)
		wg.Done()
	}()

	wg.Wait()
}
