package auth

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const claimsKey ctxKey = 1

func Require(signingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				unauthorized(w, r)
				return
			}
			claims, err := Verify(token, signingKey)
			if err != nil {
				unauthorized(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFrom(r *http.Request) *Claims {
	c, _ := r.Context().Value(claimsKey).(*Claims)
	return c
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if q := r.URL.Query().Get("jwt"); q != "" {
		return q
	}
	if c, err := r.Cookie("jwt"); err == nil {
		return c.Value
	}
	return ""
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func wantsJSON(r *http.Request) bool {
	if r.Header.Get("X-Requested-With") == "fetch" {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}
