package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fkteams/internal/app/config"
	"fkteams/internal/runtime/env"
	runtimelog "fkteams/internal/runtime/log"

	"github.com/gin-gonic/gin"
)

func TestAuthEnabledAndValidateToken(t *testing.T) {
	saveHandlerConfig(t, config.Config{})

	enabled, err := AuthEnabled()
	if err != nil {
		t.Fatalf("AuthEnabled disabled error: %v", err)
	}
	if enabled {
		t.Fatal("expected auth disabled by default")
	}

	saveHandlerConfig(t, config.Config{
		Server: config.Server{Auth: config.ServerAuth{Enabled: true}},
	})
	enabled, err = AuthEnabled()
	if err == nil {
		t.Fatal("expected missing secret error")
	}
	if enabled {
		t.Fatal("expected auth disabled when secret is missing")
	}

	saveHandlerConfig(t, config.Config{
		Server: config.Server{Auth: config.ServerAuth{
			Enabled:  true,
			Username: "alice",
			Password: "first-password",
			Secret:   "test-secret",
		}},
	})
	enabled, err = AuthEnabled()
	if err != nil {
		t.Fatalf("AuthEnabled enabled error: %v", err)
	}
	if !enabled {
		t.Fatal("expected auth enabled")
	}

	token := generateToken("alice")
	if !ValidateToken(token) {
		t.Fatal("expected generated token to be valid")
	}
	if ValidateToken("invalid") {
		t.Fatal("expected malformed token to be invalid")
	}
	if ValidateToken(token + "0") {
		t.Fatal("expected tampered token to be invalid")
	}
	if ValidateToken(signedTestToken(t, "alice", time.Now().Add(-time.Minute))) {
		t.Fatal("expected expired token to be invalid")
	}

	saveHandlerConfig(t, config.Config{
		Server: config.Server{Auth: config.ServerAuth{
			Enabled:  true,
			Username: "alice",
			Password: "second-password",
			Secret:   "test-secret",
		}},
	})
	if ValidateToken(token) {
		t.Fatal("expected password change to invalidate existing token")
	}

	token = generateToken("alice")
	saveHandlerConfig(t, config.Config{
		Server: config.Server{Auth: config.ServerAuth{
			Enabled:  true,
			Username: "bob",
			Password: "second-password",
			Secret:   "test-secret",
		}},
	})
	if ValidateToken(token) {
		t.Fatal("expected username change to invalidate existing token")
	}
}

func TestLoginHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	saveHandlerConfig(t, config.Config{
		Server: config.Server{Auth: config.ServerAuth{
			Enabled:  true,
			Username: "admin",
			Password: "secret",
			Secret:   "token-secret",
		}},
	})

	router := gin.New()
	router.POST("/login", LoginHandler())

	resp := performJSON(router, http.MethodPost, "/login", `{bad json`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad json status 400, got %d", resp.Code)
	}

	resp = performJSON(router, http.MethodPost, "/login", `{"username":"admin","password":"wrong"}`)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong password status 401, got %d", resp.Code)
	}

	resp = performJSON(router, http.MethodPost, "/login", `{"username":"admin","password":"secret"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var got Response
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := got.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", got.Data)
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected token string, got %#v", data["token"])
	}
	if !ValidateToken(token) {
		t.Fatal("expected login token to be valid")
	}
	assertAuthCookie(t, resp, token, false)

	resp = performJSON(router, http.MethodPost, "/login", `{"username":"admin","password":"secret","cookie_only":true}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected cookie-only login status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal cookie-only response: %v", err)
	}
	data, ok = got.Data.(map[string]any)
	if !ok || data["authenticated"] != true {
		t.Fatalf("expected authenticated response, got %#v", got.Data)
	}
	if _, exists := data["token"]; exists {
		t.Fatalf("cookie-only response must not expose token: %#v", data)
	}
	cookies := resp.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == "" {
		t.Fatalf("expected cookie-only login cookie, got %#v", cookies)
	}
}

func TestLoginCookieUsesSecureFlagForHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	saveHandlerConfig(t, config.Config{Server: config.Server{Auth: config.ServerAuth{
		Enabled: true, Username: "admin", Password: "secret", Secret: "token-secret",
	}}})

	router := gin.New()
	router.POST("/login", LoginHandler())
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", resp.Code, resp.Body.String())
	}
	var payload Response
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := payload.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", payload.Data)
	}
	token, _ := data["token"].(string)
	assertAuthCookie(t, resp, token, true)
}

func TestLoginHandlerRateLimitsInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	saveHandlerConfig(t, config.Config{Server: config.Server{Auth: config.ServerAuth{
		Enabled: true, Username: "admin", Password: "secret", Secret: "token-secret",
	}}})

	router := gin.New()
	router.POST("/login", LoginHandler())
	const remoteIP = "198.51.100.32"
	attemptKey := "login:" + remoteIP
	loginAttempts.Reset(attemptKey)
	t.Cleanup(func() { loginAttempts.Reset(attemptKey) })

	for i := 0; i < 8; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"admin","password":"bad"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remoteIP + ":1234"
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401: %s", i+1, resp.Code, resp.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"admin","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteIP + ":1234"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTooManyRequests || resp.Header().Get("Retry-After") == "" {
		t.Fatalf("rate-limited status = %d, Retry-After=%q: %s", resp.Code, resp.Header().Get("Retry-After"), resp.Body.String())
	}
}

func TestLogoutHandlerClearsAuthCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/logout", LogoutHandler())

	resp := performJSON(router, http.MethodPost, "/logout", `{}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("logout status = %d: %s", resp.Code, resp.Body.String())
	}
	cookies := resp.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != authCookieName || cookies[0].MaxAge >= 0 {
		t.Fatalf("expected expired auth cookie, got %#v", cookies)
	}
}

func TestRequestAuthTokenRejectsQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/token", func(c *gin.Context) {
		c.String(http.StatusOK, RequestAuthToken(c))
	})

	resp := performRequest(router, http.MethodGet, "/token?token=query-secret", nil)
	if resp.Body.String() != "" {
		t.Fatalf("query token must be ignored, got %q", resp.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "cookie-token"})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Body.String() != "cookie-token" {
		t.Fatalf("cookie token = %q, want cookie-token", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "cookie-token"})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Body.String() != "header-token" {
		t.Fatalf("header token = %q, want header-token", resp.Body.String())
	}
}

func TestLoginHandlerRejectsLoginWhenAuthDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	saveHandlerConfig(t, config.Config{})

	router := gin.New()
	router.POST("/login", LoginHandler())
	resp := performJSON(router, http.MethodPost, "/login", `{"username":"admin","password":"secret"}`)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected disabled auth status 404, got %d", resp.Code)
	}
}

func TestStreamSubscriptionAuthorizedTracksAuthChanges(t *testing.T) {
	saveHandlerConfig(t, config.Config{})
	if !streamSubscriptionAuthorized("") {
		t.Fatal("expected subscription to remain authorized when auth is disabled")
	}

	saveHandlerConfig(t, config.Config{Server: config.Server{Auth: config.ServerAuth{
		Enabled:  true,
		Username: "alice",
		Password: "first-password",
		Secret:   "test-secret",
	}}})
	token := generateToken("alice")
	if !streamSubscriptionAuthorized(token) {
		t.Fatal("expected valid token to authorize subscription")
	}
	if streamSubscriptionAuthorized("invalid") {
		t.Fatal("expected invalid token to reject subscription")
	}

	saveHandlerConfig(t, config.Config{Server: config.Server{Auth: config.ServerAuth{
		Enabled:  true,
		Username: "alice",
		Password: "second-password",
		Secret:   "test-secret",
	}}})
	if streamSubscriptionAuthorized(token) {
		t.Fatal("expected credential change to reject existing subscription token")
	}
}

func saveHandlerConfig(t *testing.T, cfg config.Config) {
	t.Helper()

	t.Setenv(env.AppDir, t.TempDir())
	t.Cleanup(func() { _ = runtimelog.Close() })
	if err := config.Save(&cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func signedTestToken(t *testing.T, username string, expiry time.Time) string {
	t.Helper()

	payload := username + "|" + expiry.Format(time.RFC3339)
	mac := hmac.New(sha256.New, getTokenSecret())
	if _, err := mac.Write([]byte(payload)); err != nil {
		t.Fatalf("write hmac payload: %v", err)
	}
	return hex.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func assertAuthCookie(t *testing.T, resp *httptest.ResponseRecorder, token string, secure bool) {
	t.Helper()
	cookies := resp.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one auth cookie, got %#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != authCookieName || cookie.Value != token {
		t.Fatalf("unexpected auth cookie: %#v", cookie)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure != secure {
		t.Fatalf("insecure auth cookie attributes: %#v", cookie)
	}
	if cookie.MaxAge != int(authTokenTTL/time.Second) {
		t.Fatalf("auth cookie MaxAge = %d, want %d", cookie.MaxAge, int(authTokenTTL/time.Second))
	}
}

func performJSON(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
