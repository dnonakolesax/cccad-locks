package auth

import (
	"log/slog"
	"net/http"
)

const (
	AccessTokenCookie  = "NTD-DNAnAT"
	RefreshTokenCookie = "NTD-DNART"
	IDTokenCookie      = "NTD-DNALT"
)

type Middleware struct {
	client *Client
	logger *slog.Logger
}

func NewMiddleware(client *Client, logger *slog.Logger) *Middleware {
	return &Middleware{
		client: client,
		logger: logger,
	}
}

func (m *Middleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || m.client == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		accessToken, ok := cookieValue(r, AccessTokenCookie)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		refreshToken, ok := cookieValue(r, RefreshTokenCookie)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		tokenData, err := m.client.Authenticate(r.Context(), accessToken, refreshToken)
		if err != nil {
			if m.logger != nil {
				m.logger.WarnContext(r.Context(), "Authentication failed", slog.String("error", err.Error()))
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		setRenewedTokenCookie(w, AccessTokenCookie, tokenData.AT)
		setRenewedTokenCookie(w, RefreshTokenCookie, tokenData.RT)
		setRenewedTokenCookie(w, IDTokenCookie, tokenData.IT)

		next.ServeHTTP(w, r.WithContext(ContextWithUserID(r.Context(), tokenData.UserID)))
	})
}

func cookieValue(r *http.Request, name string) (string, bool) {
	cookie, err := r.Cookie(name)
	if err != nil || cookie.Value == "" {
		return "", false
	}

	return cookie.Value, true
}

func setRenewedTokenCookie(w http.ResponseWriter, name string, value *string) {
	if value == nil || *value == "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    *value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
