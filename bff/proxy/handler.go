package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

		// リクエストボディを一時バッファに退避する（401 時のリトライで再送するため）
		var bodyBytes []byte
		if r.Body != nil {
			var err error
			bodyBytes, err = io.ReadAll(r.Body)
			if err != nil {
				slog.Error("Failed to read request body", "error", err)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 1 回目のプロキシ試行（レスポンスを recorder に受けて 401 を検出する）
		recorder := httptest.NewRecorder()
		rp.ServeHTTP(recorder, r)

		if recorder.Code != http.StatusUnauthorized {
			flushRecorder(w, recorder)
			return
		}

		// API が 401 を返した場合、トークンリフレッシュを試みる（ADR-0020）
		slog.Info("API returned 401, attempting token refresh", "url", r.URL.String())

		sessionCookie, err := r.Cookie(cfg.SessionCookieName)
		if err != nil {
			slog.Warn("No session cookie for token refresh, returning 401")
			flushRecorder(w, recorder)
			return
		}

		sessionData, err := redis.GetSessionValue(sessionCookie.Value)
		if err != nil || sessionData.RefreshToken == "" {
			slog.Warn("Session not found or no refresh token available", "error", err)
			flushRecorder(w, recorder)
			return
		}

		newSessionData, err := auth.RefreshAPIToken(r.Context(), sessionData)
		if err != nil {
			// セッションは削除しない（Keycloak の一時的な障害や、認可エラー等の可能性があるため）
			// TTL による自然消滅に任せる
			slog.Warn("Token refresh failed, returning original 401", "error", err)
			flushRecorder(w, recorder)
			return
		}

		if err := redis.UpdateSession(sessionCookie.Value, newSessionData); err != nil {
			slog.Error("Failed to update session after token refresh", "error", err)
			flushRecorder(w, recorder)
			return
		}

		// リフレッシュ成功 → 同一リクエストをリトライ（rewrite がセッションから新 AccessToken を取得する）
		slog.Info("Token refreshed successfully, retrying request", "url", r.URL.String())
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		rp.ServeHTTP(w, r)
	})
}

// flushRecorder は httptest.ResponseRecorder の内容を実際の ResponseWriter へコピーする。
func flushRecorder(w http.ResponseWriter, rec *httptest.ResponseRecorder) {
	for k, vs := range rec.Header() {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}
