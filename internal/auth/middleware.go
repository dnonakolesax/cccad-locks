package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
)

const (
	AccessTokenCookie  = "NTD-DNAnAT" //nolint:gosec // Cookie name, not a credential.
	RefreshTokenCookie = "NTD-DNART"  //nolint:gosec // Cookie name, not a credential.
	IDTokenCookie      = "NTD-DNALT"  //nolint:gosec // Cookie name, not a credential.
	TraceIDHeader      = "Trace_id"
)

const (
	uuidVersionMask    = 0x40
	uuidVariantMask    = 0x80
	uuidVersionBitmask = 0x0f
	uuidVariantBitmask = 0x3f
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
		if !ok && m.logger != nil {
			m.logger.DebugContext(r.Context(), "Access token cookie is missing, trying refresh token")
		}
		refreshToken, ok := cookieValue(r, RefreshTokenCookie)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		traceID := strings.TrimSpace(r.Header.Get(TraceIDHeader))
		if traceID == "" {
			traceID = newTraceID()
		}
		ctx = ContextWithTraceID(ctx, traceID)

		tokenData, err := m.client.Authenticate(ctx, accessToken, refreshToken)
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

		next.ServeHTTP(w, r.WithContext(ContextWithUserID(ctx, tokenData.UserID)))
	})
}

func newTraceID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}

	id[6] = (id[6] & uuidVersionBitmask) | uuidVersionMask
	id[8] = (id[8] & uuidVariantBitmask) | uuidVariantMask

	var encoded [36]byte
	hex.Encode(encoded[0:8], id[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], id[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], id[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], id[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], id[10:16])

	return string(encoded[:])
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
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
