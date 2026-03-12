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
)

var errNoSessionCookie = fmt.Errorf("No session cookie found, creating new session")

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func NewHandler(forwardHost string) http.Handler {
	var existedSessionId *string

	rewrite := func(request *httputil.ProxyRequest) {
		sessionID, err := request.In.Cookie("Session-Id")
		if err != nil {
			slog.Error("Error getting cookie", "error", err)
		} else {
			slog.Info("Received request with cookie", "cookie", sessionID)
		}

		// Check if the session ID exists in Redis
		hashedSessionId := hashToken(sessionID.Value)
		_, err = redis.GetSessionValue(sessionID.Value)
		if err != nil {
			slog.Info("Session not found in Redis", "sessionId", sessionID.Value)
			existedSessionId = nil
		} else {
			slog.Info("Session found in Redis", "sessionId", sessionID.Value)
			existedSessionId = &hashedSessionId
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
		response.Header.Set("Access-Control-Allow-Origin", "https://localhost:3000")
		response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if existedSessionId == nil {
			slog.Info("No session cookie found, creating new session")
		} else {
			redis.DeleteSession(*existedSessionId)
		}

		session_id := uuid.New()
		response.Header.Set("Set-Cookie", fmt.Sprintf("Session-Id= %s; Secure", session_id.String()))

		// Store the session in Redis with the client's IP address
		err := redis.SetSession(session_id.String(), response.Request.RemoteAddr)
		if err != nil {
			slog.Error("Error setting session in Redis", "error", err)
		} else {
			slog.Info("Session stored in Redis", "key", "session:"+session_id.String(), "value", response.Request.RemoteAddr)
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
