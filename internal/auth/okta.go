package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"frontend-2-v2/internal/config"
)

type OktaService struct {
	cfg    *config.Config
	client *http.Client
}

func NewOkta(cfg *config.Config) *OktaService {
	return &OktaService{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *OktaService) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevAuthBypass || s.cfg.OktaIssuer == "" {
		s.devLogin(w, r)
		return
	}

	state, err := randomString(24)
	if err != nil {
		http.Error(w, "could not generate state", http.StatusInternalServerError)
		return
	}
	setStateCookie(w, state)

	q := url.Values{}
	q.Set("client_id", s.cfg.OktaClientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("redirect_uri", s.cfg.OktaRedirectURL)
	q.Set("state", state)

	authorize := strings.TrimRight(s.cfg.OktaIssuer, "/") + "/v1/authorize?" + q.Encode()
	http.Redirect(w, r, authorize, http.StatusSeeOther)
}

func (s *OktaService) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevAuthBypass || s.cfg.OktaIssuer == "" {
		s.devLogin(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != state {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	clearStateCookie(w)

	token, err := s.exchangeCode(r.Context(), code)
	if err != nil {
		http.Error(w, "code exchange failed", http.StatusBadGateway)
		return
	}
	user, err := s.userInfo(r.Context(), token)
	if err != nil {
		http.Error(w, "userinfo failed", http.StatusBadGateway)
		return
	}

	jwt, err := Mint(s.cfg.JWTSigningKey, s.cfg.JWTIssuer, user.Email, user.Name)
	if err != nil {
		http.Error(w, "could not mint token", http.StatusInternalServerError)
		return
	}
	finishLogin(w, r, jwt)
}

func (s *OktaService) devLogin(w http.ResponseWriter, r *http.Request) {
	token, err := Mint(s.cfg.JWTSigningKey, s.cfg.JWTIssuer, "dev@infoblox.com", "Dev User")
	if err != nil {
		http.Error(w, "could not mint dev token", http.StatusInternalServerError)
		return
	}
	finishLogin(w, r, token)
}

func finishLogin(w http.ResponseWriter, r *http.Request, jwt string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    jwt,
		Path:     "/",
		HttpOnly: false,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(TokenLifetime.Seconds()),
	})
	http.Redirect(w, r, "/?jwt="+url.QueryEscape(jwt), http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "jwt",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

type oktaTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

type oktaUserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *OktaService) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", s.cfg.OktaRedirectURL)
	form.Set("client_id", s.cfg.OktaClientID)
	form.Set("client_secret", s.cfg.OktaClientSecret)

	endpoint := strings.TrimRight(s.cfg.OktaIssuer, "/") + "/v1/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("okta token endpoint %d: %s", resp.StatusCode, string(body))
	}
	var tr oktaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", errors.New("okta returned empty access_token")
	}
	return tr.AccessToken, nil
}

func (s *OktaService) userInfo(ctx context.Context, accessToken string) (*oktaUserInfo, error) {
	endpoint := strings.TrimRight(s.cfg.OktaIssuer, "/") + "/v1/userinfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("okta userinfo %d", resp.StatusCode)
	}
	var u oktaUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func setStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/auth",
		MaxAge: -1,
	})
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
