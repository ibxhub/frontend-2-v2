package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequire_RejectsMissingToken(t *testing.T) {
	mw := Require(testKey)
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("downstream handler must not be called when token is missing")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequire_AcceptsValidBearer(t *testing.T) {
	tok, err := Mint(testKey, "frontend-2-v2", "alice@infoblox.com", "Alice")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	mw := Require(testKey)
	var gotEmail string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := ClaimsFrom(r); c != nil {
			gotEmail = c.Email
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotEmail != "alice@infoblox.com" {
		t.Errorf("claims.Email = %q, want alice@infoblox.com", gotEmail)
	}
}

func TestRequire_AcceptsCookie(t *testing.T) {
	tok, err := Mint(testKey, "frontend-2-v2", "alice@infoblox.com", "Alice")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	mw := Require(testKey)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tok})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequire_HtmlRequestRedirects(t *testing.T) {
	mw := Require(testKey)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("downstream handler must not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/auth/login" {
		t.Errorf("Location = %q, want /auth/login", loc)
	}
}

func TestRequire_RejectsInvalidToken(t *testing.T) {
	mw := Require(testKey)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("downstream handler must not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
