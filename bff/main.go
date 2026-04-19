package main

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"bff/auth"
	"bff/proxy"
	"bff/setup"

	"github.com/go-chi/chi/v5"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(jsonHandler))

	cfg := setup.GetConfig()

	// Validate configuration at startup
	if err := cfg.Validate(); err != nil {
		slog.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Configuration validated successfully")

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		slog.Info("Starting BFF server", "addr", cfg.BFFListenAddr)

		// Chi router setup
		r := chi.NewRouter()

		// Health check endpoint
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Health check", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
		})

		// Authentication endpoints
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", auth.LoginHandler)
			r.Get("/callback", auth.CallbackHandler)
			r.Get("/me", auth.MeHandler)
			r.Post("/logout", auth.LogoutHandler)
		})

		// Proxy handler (catch-all for unmatched routes)
		r.Handle("/*", proxy.NewHandler(cfg.ProxyTarget))

		http.ListenAndServe(cfg.BFFListenAddr, r)
		wg.Done()
	}()

	wg.Wait()
}
