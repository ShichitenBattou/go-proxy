package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received login request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
	base := "http://localhost:8082/realms/myrealm/protocol/openid-connect/auth"
	params := url.Values{
		"response_type": {"id_token"},
		"client_id":     {"bff"},
		"redirect_uri":  {"https://localhost/auth/callback"},
	}
	kcLoginUrl := fmt.Sprintf("%s?%s", base, params.Encode())
	slog.Info("Redirecting to login page", "url", kcLoginUrl, "query", params.Get("response_type"), "client_id", params.Get("client_id"), "redirect_uri", params.Get("redirect_uri"), "rawQuery", params.Encode())
	http.Redirect(w, r, kcLoginUrl, http.StatusSeeOther)
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received callback request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
	fmt.Fprint(w, "Callback endpoint")
	w.WriteHeader(http.StatusNotImplemented)
}
