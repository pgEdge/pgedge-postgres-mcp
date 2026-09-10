/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// codeRecord captures what the fake authorisation server remembers about
// an issued authorisation code, so the token endpoint can validate PKCE
// and the redirect URI on redemption.
type codeRecord struct {
	challenge   string
	redirectURI string
	clientID    string
	subject     string
}

// fakeOAuthServer is a minimal stand-in for internal/oauth's HTTP surface,
// implementing just enough of RFC 8414 discovery, RFC 7591 dynamic
// registration, the authorisation code grant with PKCE, the device
// grant, refresh, and revocation for OAuthClient's tests to drive against.
type fakeOAuthServer struct {
	t   *testing.T
	srv *httptest.Server

	mu              sync.Mutex
	codes           map[string]*codeRecord
	refreshTokens   map[string]bool // refresh token -> still valid
	revoked         []string
	clientCounter   int
	mismatchState   bool   // authorize redirects with the wrong state
	authorizeError  string // authorize redirects with this error code
	deviceApproveAt int    // number of polls before the device code is approved
	devicePolls     int
	deviceUserCode  string
	deviceClientID  string
	deviceApproved  bool
}

func newFakeOAuthServer(t *testing.T) *fakeOAuthServer {
	t.Helper()
	f := &fakeOAuthServer{
		t:             t,
		codes:         map[string]*codeRecord{},
		refreshTokens: map[string]bool{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", f.handleMetadata)
	mux.HandleFunc("/oauth/register", f.handleRegister)
	mux.HandleFunc("/oauth/authorize", f.handleAuthorize)
	mux.HandleFunc("/oauth/token", f.handleToken)
	mux.HandleFunc("/oauth/device", f.handleDevice)
	mux.HandleFunc("/oauth/revoke", f.handleRevoke)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeOAuthServer) handleMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(oauthMetadata{
		Issuer:                      f.srv.URL,
		AuthorizationEndpoint:       f.srv.URL + "/oauth/authorize",
		TokenEndpoint:               f.srv.URL + "/oauth/token",
		RegistrationEndpoint:        f.srv.URL + "/oauth/register",
		DeviceAuthorizationEndpoint: f.srv.URL + "/oauth/device",
		RevocationEndpoint:          f.srv.URL + "/oauth/revoke",
	})
}

func (f *fakeOAuthServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.mu.Lock()
	f.clientCounter++
	clientID := fmt.Sprintf("client-%d", f.clientCounter)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"client_id": clientID})
}

// handleAuthorize simulates an instant login: rather than rendering a
// login page, it immediately redirects back to redirect_uri with a fresh
// authorisation code, as if the resource owner had already signed in.
// When mismatchState is set, the redirect carries a different state than
// the one the request presented, simulating a CSRF-style attack (or a
// broken client) for TestLoopbackRejectsWrongState.
func (f *fakeOAuthServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirectURI := q.Get("redirect_uri")
	state := q.Get("state")

	f.mu.Lock()
	code := fmt.Sprintf("code-%d", len(f.codes)+1)
	f.codes[code] = &codeRecord{
		challenge:   q.Get("code_challenge"),
		redirectURI: redirectURI,
		clientID:    q.Get("client_id"),
		subject:     "alice",
	}
	mismatch := f.mismatchState
	authorizeError := f.authorizeError
	f.mu.Unlock()

	if mismatch {
		state = "wrong-state"
	}

	dest, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	dq := dest.Query()
	if authorizeError != "" {
		dq.Set("error", authorizeError)
		dq.Set("state", state)
		dest.RawQuery = dq.Encode()
		http.Redirect(w, r, dest.String(), http.StatusFound)
		return
	}
	dq.Set("code", code)
	dq.Set("state", state)
	dest.RawQuery = dq.Encode()
	http.Redirect(w, r, dest.String(), http.StatusFound)
}

func (f *fakeOAuthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		f.handleAuthCodeGrant(w, r)
	case "refresh_token":
		f.handleRefreshGrant(w, r)
	case "urn:ietf:params:oauth:grant-type:device_code":
		f.handleDeviceGrant(w, r)
	default:
		writeTokenError(w, "unsupported_grant_type", "")
	}
}

func (f *fakeOAuthServer) handleAuthCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.PostFormValue("code")
	verifier := r.PostFormValue("code_verifier")
	redirectURI := r.PostFormValue("redirect_uri")

	f.mu.Lock()
	rec, ok := f.codes[code]
	if ok {
		delete(f.codes, code)
	}
	f.mu.Unlock()

	if !ok {
		writeTokenError(w, "invalid_grant", "unknown code")
		return
	}
	if rec.redirectURI != redirectURI {
		writeTokenError(w, "invalid_grant", "redirect_uri mismatch")
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if challenge != rec.challenge {
		writeTokenError(w, "invalid_grant", "verifier mismatch")
		return
	}

	f.issueTokens(w)
}

func (f *fakeOAuthServer) handleRefreshGrant(w http.ResponseWriter, r *http.Request) {
	refresh := r.PostFormValue("refresh_token")
	f.mu.Lock()
	valid := f.refreshTokens[refresh]
	f.mu.Unlock()
	if !valid {
		writeTokenError(w, "invalid_grant", "unknown refresh token")
		return
	}
	f.issueTokens(w)
}

func (f *fakeOAuthServer) handleDeviceGrant(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.devicePolls++
	approve := f.devicePolls >= f.deviceApproveAt
	if approve {
		f.deviceApproved = true
	}
	f.mu.Unlock()

	if !approve {
		writeTokenError(w, "authorization_pending", "")
		return
	}
	f.issueTokens(w)
}

// issueTokens writes a fresh access/refresh token pair, remembering the
// refresh token as valid so later refresh and revoke requests can find
// it.
func (f *fakeOAuthServer) issueTokens(w http.ResponseWriter) {
	f.mu.Lock()
	f.clientCounter++
	access := fmt.Sprintf("access-%d", f.clientCounter)
	refresh := fmt.Sprintf("refresh-%d", f.clientCounter)
	f.refreshTokens[refresh] = true
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refresh,
		"scope":         "mcp",
	})
}

func (f *fakeOAuthServer) handleDevice(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	f.deviceUserCode = "ABCD-1234"
	f.deviceClientID = r.PostFormValue("client_id")
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"device_code":               "devicecode-1",
		"user_code":                 "ABCD-1234",
		"verification_uri":          f.srv.URL + "/oauth/device/verify",
		"verification_uri_complete": f.srv.URL + "/oauth/device/verify?user_code=ABCD-1234",
		"expires_in":                600,
		"interval":                  1,
	})
}

func (f *fakeOAuthServer) handleRevoke(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token := r.PostFormValue("token")
	f.mu.Lock()
	f.revoked = append(f.revoked, token)
	delete(f.refreshTokens, token)
	f.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func writeTokenError(w http.ResponseWriter, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": desc})
}

// loopbackOpenURL performs a GET on rawURL and follows the redirect it
// receives back to the CLI's own loopback listener, exactly as a real
// browser completing an instant login would, letting the request reach
// OAuthClient's callback handler.
func loopbackOpenURL(rawURL string) error {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func TestDiscoverOAuthAbsent(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	_, err := DiscoverOAuth(context.Background(), http.DefaultClient, srv.URL)
	if !errors.Is(err, ErrNoOAuth) {
		t.Fatalf("expected ErrNoOAuth, got %v", err)
	}
}

func TestDiscoverOAuthPresent(t *testing.T) {
	f := newFakeOAuthServer(t)

	meta, err := DiscoverOAuth(context.Background(), http.DefaultClient, f.srv.URL)
	if err != nil {
		t.Fatalf("DiscoverOAuth: %v", err)
	}
	if meta.Issuer != f.srv.URL {
		t.Errorf("Issuer = %q, want %q", meta.Issuer, f.srv.URL)
	}
	if meta.TokenEndpoint != f.srv.URL+"/oauth/token" {
		t.Errorf("TokenEndpoint = %q", meta.TokenEndpoint)
	}
	if meta.RegistrationEndpoint != f.srv.URL+"/oauth/register" {
		t.Errorf("RegistrationEndpoint = %q", meta.RegistrationEndpoint)
	}
	if meta.DeviceAuthorizationEndpoint != f.srv.URL+"/oauth/device" {
		t.Errorf("DeviceAuthorizationEndpoint = %q", meta.DeviceAuthorizationEndpoint)
	}
	if meta.RevocationEndpoint != f.srv.URL+"/oauth/revoke" {
		t.Errorf("RevocationEndpoint = %q", meta.RevocationEndpoint)
	}
}

func TestLoopbackLoginRoundTrip(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		OpenURL:   loopbackOpenURL,
		Prompt:    func(string) {},
	}

	if err := oc.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	token := oc.Token()
	if token == "" {
		t.Fatal("Token() returned empty string after login")
	}

	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file mode = %o, want 0600", perm)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	var cache map[string]oauthCacheEntry
	if err := yaml.Unmarshal(data, &cache); err != nil {
		t.Fatalf("unmarshal cache: %v", err)
	}
	entry, ok := cache[f.srv.URL]
	if !ok {
		t.Fatalf("cache missing entry for issuer %q: %v", f.srv.URL, cache)
	}
	if entry.AccessToken != token {
		t.Errorf("cached access token = %q, want %q", entry.AccessToken, token)
	}
	if entry.RefreshToken == "" {
		t.Error("cached refresh token is empty")
	}
}

func TestLoopbackRejectsWrongState(t *testing.T) {
	f := newFakeOAuthServer(t)
	f.mismatchState = true
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		OpenURL:   loopbackOpenURL,
		Prompt:    func(string) {},
	}

	err := oc.Login(context.Background())
	if err == nil {
		t.Fatal("expected Login to fail on state mismatch")
	}

	if _, statErr := os.Stat(cachePath); statErr == nil {
		t.Error("cache file was written despite the state mismatch")
	}
}

// twiceOpenURL performs a GET on rawURL twice, following each redirect
// back to the CLI's loopback listener, so a test can simulate a second
// request reaching the callback (a double-clicked link, a browser retry,
// or a stray probe) after the first has already been handled. It records
// both responses' status and body for the test to inspect.
type recordedResponse struct {
	status int
	body   string
}

func twiceOpenURL(responses *[]recordedResponse) func(string) error {
	return func(rawURL string) error {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		}
		for i := 0; i < 2; i++ {
			resp, err := client.Get(rawURL)
			if err != nil {
				return err
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			*responses = append(*responses, recordedResponse{status: resp.StatusCode, body: string(body)})
		}
		return nil
	}
}

// onceRecordingOpenURL is twiceOpenURL for a single request, for tests
// that need to inspect the page the callback rendered.
func onceRecordingOpenURL(responses *[]recordedResponse) func(string) error {
	return func(rawURL string) error {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
		}
		resp, err := client.Get(rawURL)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		*responses = append(*responses, recordedResponse{status: resp.StatusCode, body: string(body)})
		return nil
	}
}

func TestLoopbackSecondCallbackGetsAlreadyHandled(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	var responses []recordedResponse
	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		OpenURL:   twiceOpenURL(&responses),
		Prompt:    func(string) {},
	}

	if err := oc.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if oc.Token() == "" {
		t.Fatal("Token() returned empty string after login")
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses from the loopback listener, got %d", len(responses))
	}
	first, second := responses[0], responses[1]
	if strings.Contains(first.body, "already been handled") {
		t.Errorf("first callback got the already-handled page: %q", first.body)
	}
	if !strings.Contains(second.body, "already been handled") {
		t.Errorf("second callback did not get the already-handled page: %q", second.body)
	}
	if second.status != http.StatusOK {
		t.Errorf("second callback status = %d, want 200", second.status)
	}
}

// TestLoopbackErrorPageEscapesServerText checks that an error code from
// the authorisation server cannot inject markup into the loopback
// callback page, which is rendered through html/template precisely so
// that it cannot.
func TestLoopbackErrorPageEscapesServerText(t *testing.T) {
	f := newFakeOAuthServer(t)
	f.authorizeError = `<script>alert(1)</script>`
	forceLoopbackEnvironment(t)

	var responses []recordedResponse
	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: filepath.Join(t.TempDir(), "oauth-tokens.yaml"),
		OpenURL:   onceRecordingOpenURL(&responses),
		Prompt:    func(string) {},
	}

	if err := oc.Login(context.Background()); err == nil {
		t.Fatal("expected Login to fail when the server denies the request")
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response from the loopback listener, got %d", len(responses))
	}
	body := responses[0].body
	if strings.Contains(body, "<script>") {
		t.Errorf("callback page carries unescaped markup: %q", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("callback page does not carry the escaped error code: %q", body)
	}
}

func TestDeviceLoginWhenNoBrowser(t *testing.T) {
	f := newFakeOAuthServer(t)
	f.deviceApproveAt = 3
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")

	var prompts []string
	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		NoBrowser: true,
		Prompt:    func(msg string) { prompts = append(prompts, msg) },
	}
	// Avoid a real sleep between polls in the test.
	oc.sleep = func(time.Duration) {}

	if err := oc.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if len(prompts) == 0 {
		t.Fatal("Prompt was never called")
	}
	joined := strings.Join(prompts, "\n")
	if !strings.Contains(joined, "ABCD-1234") {
		t.Errorf("prompt %q does not mention the user code", joined)
	}
	if !strings.Contains(joined, "device/verify") {
		t.Errorf("prompt %q does not mention the verification URL", joined)
	}

	if oc.Token() == "" {
		t.Fatal("Token() returned empty string after device login")
	}

	f.mu.Lock()
	polls := f.devicePolls
	f.mu.Unlock()
	if polls < 3 {
		t.Errorf("expected at least 3 polls, got %d", polls)
	}
}

func TestTokenRefreshesNearExpiry(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")

	now := time.Now()
	cache := map[string]oauthCacheEntry{
		f.srv.URL: {
			ClientID:     "client-1",
			AccessToken:  "stale-access",
			RefreshToken: "refresh-seed",
			ExpiresAt:    now.Add(30 * time.Second),
		},
	}
	f.refreshTokens["refresh-seed"] = true
	writeTestCache(t, cachePath, cache)

	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		Now:       func() time.Time { return now },
	}

	token := oc.Token()
	if token == "" {
		t.Fatal("Token() returned empty string")
	}
	if token == "stale-access" {
		t.Error("Token() returned the stale access token instead of refreshing")
	}
}

func TestLogoutRevokesAndClears(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		OpenURL:   loopbackOpenURL,
		Prompt:    func(string) {},
	}
	if err := oc.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	f.mu.Lock()
	refreshToken := ""
	for tok := range f.refreshTokens {
		refreshToken = tok
	}
	f.mu.Unlock()

	if err := oc.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	f.mu.Lock()
	revoked := append([]string(nil), f.revoked...)
	f.mu.Unlock()
	found := false
	for _, tok := range revoked {
		if tok == refreshToken {
			found = true
		}
	}
	if !found {
		t.Errorf("revoke endpoint did not receive refresh token %q, got %v", refreshToken, revoked)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	var cache map[string]oauthCacheEntry
	if err := yaml.Unmarshal(data, &cache); err != nil {
		t.Fatalf("unmarshal cache: %v", err)
	}
	if _, ok := cache[f.srv.URL]; ok {
		t.Error("cache still has an entry for the issuer after logout")
	}
}

func TestCacheIsPerIssuer(t *testing.T) {
	f1 := newFakeOAuthServer(t)
	f2 := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	for _, f := range []*fakeOAuthServer{f1, f2} {
		oc := &OAuthClient{
			BaseURL:   f.srv.URL,
			CachePath: cachePath,
			OpenURL:   loopbackOpenURL,
			Prompt:    func(string) {},
		}
		if err := oc.Login(context.Background()); err != nil {
			t.Fatalf("Login (%s): %v", f.srv.URL, err)
		}
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	var cache map[string]oauthCacheEntry
	if err := yaml.Unmarshal(data, &cache); err != nil {
		t.Fatalf("unmarshal cache: %v", err)
	}
	if len(cache) != 2 {
		t.Fatalf("expected 2 cache entries, got %d: %v", len(cache), cache)
	}
	if _, ok := cache[f1.srv.URL]; !ok {
		t.Errorf("missing entry for %q", f1.srv.URL)
	}
	if _, ok := cache[f2.srv.URL]; !ok {
		t.Errorf("missing entry for %q", f2.srv.URL)
	}
}

// TestSaveOAuthCacheFixesLoosePermissions confirms that saving over an
// existing, looser-mode cache file (and directory) tightens the mode
// back to 0600/0700 rather than leaving whatever mode the file already
// had, which is what os.WriteFile alone would do: it only applies the
// given mode when it creates the file.
func TestSaveOAuthCacheFixesLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions do not apply on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "oauth-tokens.yaml")

	if err := os.WriteFile(path, []byte("stale: true\n"), 0o644); err != nil {
		t.Fatalf("seed cache file: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("loosen cache directory mode: %v", err)
	}

	cache := map[string]oauthCacheEntry{
		"https://issuer.example": {AccessToken: "tok"},
	}
	if err := saveOAuthCache(path, cache); err != nil {
		t.Fatalf("saveOAuthCache: %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file mode = %o, want 0600", perm)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat cache directory: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("cache directory mode = %o, want 0700", perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	var roundTripped map[string]oauthCacheEntry
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal cache: %v", err)
	}
	if roundTripped["https://issuer.example"].AccessToken != "tok" {
		t.Errorf("cache content = %v, want the freshly saved entry", roundTripped)
	}
}

// forceLoopbackEnvironment makes shouldUseDeviceFlow's Linux heuristic
// ("no DISPLAY or WAYLAND_DISPLAY means no browser") report a GUI
// session, so a test that exercises the loopback flow does so even when
// it runs headless (as it does in CI). t.Setenv restores the previous
// value automatically.
func forceLoopbackEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DISPLAY", ":0")
}

func writeTestCache(t *testing.T, path string, cache map[string]oauthCacheEntry) {
	t.Helper()
	data, err := yaml.Marshal(cache)
	if err != nil {
		t.Fatalf("marshal test cache: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write test cache: %v", err)
	}
}
