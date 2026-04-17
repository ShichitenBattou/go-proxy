package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"

	"github.com/google/uuid"

	"bff/redis"
	"bff/setup"
)

var errNoSessionCookie = fmt.Errorf("No session cookie found, creating new session")

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func NewHandler(forwardHost string) http.Handler {
	cfg := setup.GetConfig()
	var existedSessionId *string

	rewrite := func(request *httputil.ProxyRequest) {
		sessionID, err := request.In.Cookie(cfg.SessionCookieName)
		if err != nil {
			slog.Error("Error getting cookie", "error", err)
		} else {
			slog.Info("Received request with cookie", "cookie", sessionID)
		}

		// Check if the session ID exists in Redis
		hashedSessionId := hashToken(sessionID.Value)
		sessionData, err := redis.GetSessionValue(sessionID.Value)
		if err != nil {
			slog.Info("Session not found in Redis", "sessionId", sessionID.Value)
			existedSessionId = nil
		} else {
			slog.Info("Session found in Redis", "sessionId", sessionID.Value)
			existedSessionId = &hashedSessionId

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

		// Session rotation: delete old session and create new one
		var oldSessionData redis.SessionData
		if existedSessionId == nil {
			slog.Info("No existing session found, skipping session rotation")
			return nil
		} else {
			// Retrieve old session data before deletion
			sessionCookie, _ := response.Request.Cookie(cfg.SessionCookieName)
			if sessionCookie != nil {
				oldSessionData, _ = redis.GetSessionValue(sessionCookie.Value)
			}
			redis.DeleteSession(*existedSessionId)
		}

		// Create new session with rotated ID
		newSessionID := uuid.New()
		cookieValue := fmt.Sprintf("%s=%s", cfg.SessionCookieName, newSessionID.String())
		if cfg.SessionCookieSecure {
			cookieValue += "; Secure"
		}
		response.Header.Set("Set-Cookie", cookieValue)

		// Store the session with existing SessionData (rotation)
		err := redis.SetSession(newSessionID.String(), oldSessionData)
		if err != nil {
			slog.Error("Error setting session in Redis", "error", err)
		} else {
			slog.Info("Session rotated in Redis", "key", "session:"+newSessionID.String(), "userId", oldSessionData.UserID)
		}

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
