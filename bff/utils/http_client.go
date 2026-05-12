package utils

import (
	"bff/setup"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
	"os"
	"sync"
)

var (
	internalClient *http.Client
	once sync.Once
)

func GetInternalHTTPClient() *http.Client {
	once.Do(func() {
		internalClient = createHTTPClientWithCustomCA()
	})
	return internalClient
}

func createHTTPClientWithCustomCA() *http.Client {
	cfg := setup.GetConfig()
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		slog.Error("Failed to load system root CAs", "error", err)
	}

	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	caPEM, err := os.ReadFile(cfg.RootCAFile)
	if err != nil {
		slog.Error("Failed to read root CA certificate", "path", cfg.RootCAFile, "error", err)
	} else {
		if ok := rootCAs.AppendCertsFromPEM(caPEM); !ok {
			slog.Warn("No certificates were appended, but no error was returned")
		} else {
			slog.Info("Successfully loaded root CA certificate", "path", cfg.RootCAFile)
		}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		},
	}
}