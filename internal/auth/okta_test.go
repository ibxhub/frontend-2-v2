package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frontend-2-v2/internal/config"
)

func devCfg() *config.Config {
	return &config.Config{
		JWTSigningKey: testKey,
		JWTIssuer:     "frontend-2-v2-test",
		DevAuthBypass: true,
	}
}

func TestOkta_DevLogin_IssuesJWT(t *testing.T) {
	svc := NewOkta(devCfg())
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	svc.LoginHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/?jwt=") {
		t.Errorf("Location = %q, want /?jwt=...", loc)
	}

	var jwtCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "jwt" {
			jwtCookie = c
		}
	}
	if jwtCookie == nil {
		t.Fatal("jwt cookie not set")
	}
	claims, err := Verify(jwtCookie.Value, testKey)
	if err != nil {
		t.Fatalf("Verify dev token: %v", err)
	}
	if claims.Email == "" {
		t.Error("dev claims missing email")
	}
}

func TestOkta_DevCallback_AlsoMints(t *testing.T) {
	svc := NewOkta(devCfg())
	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	rec := httptest.NewRecorder()
	svc.CallbackHandler(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
}

func TestOkta_CallbackProd_StateMismatch(t *testing.T) {
	cfg := &config.Config{
		JWTSigningKey: testKey,
		JWTIssuer:     "frontend-2-v2",
		OktaIssuer:    "https://example.okta.com/oauth2/default",
		OktaClientID:  "client",
		OktaClientSecret: "secret",
		OktaRedirectURL:  "https://app.test/auth/callback",
	}
	svc := NewOkta(cfg)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=mismatch", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "real-state"})
	rec := httptest.NewRecorder()
	svc.CallbackHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 on state mismatch", rec.Code)
	}
}

func TestOkta_CallbackProd_MissingCode(t *testing.T) {
	cfg := &config.Config{
		JWTSigningKey: testKey,
		OktaIssuer:    "https://example.okta.com",
		OktaClientID:  "x",
	}
	svc := NewOkta(cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	rec := httptest.NewRecorder()
	svc.CallbackHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when code missing", rec.Code)
	}
}

func TestOkta_LoginProd_RedirectsToAuthorize(t *testing.T) {
	cfg := &config.Config{
		JWTSigningKey: testKey,
		OktaIssuer:    "https://example.okta.com/oauth2/default",
		OktaClientID:  "client-abc",
		OktaRedirectURL: "https://app.test/auth/callback",
	}
	svc := NewOkta(cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	svc.LoginHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://example.okta.com/oauth2/default/v1/authorize?") {
		t.Errorf("Location = %q, want authorize URL", loc)
	}
	if !strings.Contains(loc, "client_id=client-abc") {
		t.Errorf("Location missing client_id: %q", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("Location missing state: %q", loc)
	}

	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}
	if stateCookie == nil || stateCookie.Value == "" {
		t.Error("oauth_state cookie should be set")
	}
}

func TestLogoutHandler_ClearsCookieAndRedirects(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: "anything"})
	rec := httptest.NewRecorder()
	LogoutHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/auth/login" {
		t.Errorf("Location = %q, want /auth/login", loc)
	}

	var jwtCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "jwt" {
			jwtCookie = c
		}
	}
	if jwtCookie == nil || jwtCookie.MaxAge >= 0 {
		t.Error("logout should clear the jwt cookie (MaxAge < 0)")
	}
}

func TestOkta_CallbackProd_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			http.Error(w, "wrong grant_type", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-okta-token","token_type":"Bearer"}`))
	})
	mux.HandleFunc("/v1/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-okta-token" {
			http.Error(w, "no bearer", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"alice@infoblox.com","name":"Alice"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &config.Config{
		JWTSigningKey:    testKey,
		JWTIssuer:        "frontend-2-v2",
		OktaIssuer:       srv.URL,
		OktaClientID:     "test-client",
		OktaClientSecret: "test-secret",
		OktaRedirectURL:  "https://app.test/auth/callback",
	}
	svc := NewOkta(cfg)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=expected", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "expected"})
	rec := httptest.NewRecorder()
	svc.CallbackHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/?jwt=") {
		t.Errorf("Location = %q, want /?jwt=...", loc)
	}
}

func TestOkta_CallbackProd_TokenEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{
		JWTSigningKey:    testKey,
		OktaIssuer:       srv.URL,
		OktaClientID:     "x",
		OktaClientSecret: "y",
		OktaRedirectURL:  "https://app.test/cb",
	}
	svc := NewOkta(cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=x", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "x"})
	rec := httptest.NewRecorder()
	svc.CallbackHandler(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 on token endpoint failure", rec.Code)
	}
}

func TestRandomString_UniqueAndLength(t *testing.T) {
	a, err := randomString(24)
	if err != nil {
		t.Fatalf("randomString: %v", err)
	}
	b, err := randomString(24)
	if err != nil {
		t.Fatalf("randomString: %v", err)
	}
	if a == b {
		t.Error("two random strings should differ")
	}
	if len(a) < 24 {
		t.Errorf("randomString output too short: %q", a)
	}
}
