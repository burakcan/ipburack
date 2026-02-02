package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

type AuthMiddleware struct {
	apiKey []byte
}

func NewAuth(apiKey string) *AuthMiddleware {
	return &AuthMiddleware{
		apiKey: []byte(apiKey),
	}
}

func (a *AuthMiddleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	// No API key configured = auth disabled
	if len(a.apiKey) == 0 {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")

		// Constant-time comparison prevents timing attacks
		if subtle.ConstantTimeCompare([]byte(key), a.apiKey) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid or missing API key"})
			return
		}

		next(w, r)
	}
}
