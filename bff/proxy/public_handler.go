package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/google/uuid"

	"bff/setup"
)

func NewPublicHandler(forwardHost string) http.Handler {
	cfg := setup.GetConfig()

	rewrite := func(request *httputil.ProxyRequest) {
		request.Out.Header["X-Forwarded-For"] = request.In.Header["X-Forwarded-For"]
		request.Out.URL.Scheme = "http"
		request.Out.URL.Host = forwardHost
		request.Out.Header.Set("Request-Id", uuid.New().String())
		request.Out.URL.Path = request.In.URL.Path
		request.SetXForwarded()
		slog.Info("Proxying public request", "url", request.Out.URL.String(), "requestedHost", request.In.Host, "ip", request.In.RemoteAddr)
	}

	modifyResponse := func(response *http.Response) error {
		slog.Info("Received public response", "statusCode", response.StatusCode, "url", response.Request.URL.String())
		response.Header.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
		response.Header.Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)
		return nil
	}

	errorHandler := func(writer http.ResponseWriter, request *http.Request, err error) {
		slog.Error("Error proxying public request", "error", err, "url", request.URL.String())
		writer.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(writer, "Bad Gateway")
	}

	rp := &httputil.ReverseProxy{Rewrite: rewrite, ModifyResponse: modifyResponse, ErrorHandler: errorHandler}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received public API request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
		rp.ServeHTTP(w, r)
	})
}
