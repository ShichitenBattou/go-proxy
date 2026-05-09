package proxy

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"

	"github.com/google/uuid"

	"bff/auth"
	"bff/redis"
	"bff/setup"
)

var errNoSessionCookie = fmt.Errorf("No session cookie found, creating new session")

func NewHandler(forwardHost string) http.Handler {
	cfg := setup.GetConfig()

	rewrite := func(request *httputil.ProxyRequest) {
		sessionID, err := request.In.Cookie(cfg.SessionCookieName)
		if err != nil {
			slog.Error("Error getting cookie", "error", err)
		} else {
			slog.Info("Received request with cookie", "cookie", sessionID)
		}

		// Check if the session ID exists in Redis
		sessionData, err := redis.GetSessionValue(sessionID.Value)
		if err != nil {
			slog.Info("Session not found in Redis", "sessionId", sessionID.Value)
		} else {
			slog.Info("Session found in Redis", "sessionId", sessionID.Value)

			// 未プロビジョニングの場合、転送前に再試行する
			if !sessionData.Provisioned {
				slog.Info("Session not provisioned, attempting re-provisioning", "sessionId", sessionID.Value)
				if err := auth.ProvisionUser(request.In.Context(), sessionData.AccessToken, forwardHost); err != nil {
					slog.Warn("Re-provisioning failed, forwarding request anyway", "error", err)
				} else {
					sessionData.Provisioned = true
					if err := redis.UpdateSession(sessionID.Value, sessionData); err != nil {
						slog.Warn("Failed to update session provisioned flag", "error", err)
					}
				}
			}

			// Add Authorization header with AccessToken
			if sessionData.AccessToken != "" {
				request.Out.Header.Set("Authorization", "Bearer "+sessionData.AccessToken)
			} else {
				slog.Warn("AccessToken is empty in session", "sessionId", sessionID.Value)
			}
		}

		request.Out.Header["X-Forwarded-For"] = request.In.Header["X-Forwarded-For"]
		request.Out.URL.Scheme = "http"
		request.Out.URL.Host = forwardHost
		request.Out.Header.Set("Request-Id", uuid.New().String())
		urlPath := strings.TrimPrefix(request.In.URL.Path, "/api")
		if urlPath == "" || urlPath[0] != '/' {
			urlPath = "/" + urlPath
		}
		request.Out.URL.Path = path.Clean(urlPath)
		slog.Debug(request.In.URL.Path[len("/api/"):])
		request.SetXForwarded()
		slog.Info("Proxying request", "url", request.Out.URL.String(), "requestedHost", request.In.Host, "ip", request.In.RemoteAddr)
	}

	modifyResponse := func(response *http.Response) error {
		slog.Info("Received response", "statusCode", response.StatusCode, "url", response.Request.URL.String())
		response.Header.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
		response.Header.Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)
		return nil
	}

	errorHandler := func(writer http.ResponseWriter, request *http.Request, err error) {
		if errors.Is(err, errNoSessionCookie) {
			writer.WriteHeader(http.StatusUnauthorized)
		}

		slog.Error("Error proxying request", "error", err, "url", request.URL.String())
		writer.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(writer, "Bad Gateway")
	}

	rp := &httputil.ReverseProxy{Rewrite: rewrite, ModifyResponse: modifyResponse, ErrorHandler: errorHandler}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received API request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
		rp.ServeHTTP(w, r)
	})
}
