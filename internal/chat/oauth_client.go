/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package chat's oauth_client.go implements the CLI side of the OAuth
// flows the server offers in internal/oauth: discovery of the
// authorisation server's metadata (RFC 8414), dynamic client registration
// (RFC 7591), the authorisation code grant with PKCE via a loopback
// redirect (RFC 6749 + RFC 7636), the device authorisation grant for
// headless or browser-less sessions (RFC 8628), refresh, and revocation
// (RFC 7009). Tokens are cached on disk, keyed by issuer, so a session
// need not re-authenticate every run.
package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// TokenSource returns the bearer token to present on the next request. It
// is consulted on every outgoing call, rather than once at connection
// time, so an implementation such as OAuthClient.Token can transparently
// refresh an expiring token.
type TokenSource func() string

// oauthDeviceGrantType is the device grant's RFC 8628 section 3.4 grant
// type identifier, matching the constant of the same value in
// internal/oauth.
const oauthDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// oauthLoopbackRedirectPath is the path component of the loopback
// redirect URI registered with the authorisation server and used on the
// local listener.
const oauthLoopbackRedirectPath = "/callback"

// oauthLoginTimeout bounds how long the loopback flow waits for the
// browser to complete the login and redirect back.
const oauthLoginTimeout = 5 * time.Minute

// oauthTokenRequestTimeout bounds a single token, registration or
// revocation request made outside of the interactive login flows (for
// example, a background refresh triggered by Token).
const oauthTokenRequestTimeout = 15 * time.Second

// oauthRefreshSkew is how far ahead of the cached expiry time Token
// refreshes proactively, so a request begun just before expiry does not
// race the server's own clock.
const oauthRefreshSkew = 60 * time.Second

// ErrNoOAuth is returned by DiscoverOAuth when the server does not
// advertise an OAuth authorisation server at the well-known metadata
// path (a 404), which callers use to fall back to a legacy auth mode.
var ErrNoOAuth = errors.New("server does not advertise OAuth")

// errOpenBrowserFailed wraps a failure from OAuthClient.OpenURL, letting
// Login distinguish "could not launch a browser" (fall back to the
// device flow) from every other loopback failure (report it).
var errOpenBrowserFailed = errors.New("failed to open browser")

// oauthMetadata is the subset of the RFC 8414 authorisation server
// metadata document the CLI needs.
type oauthMetadata struct {
	Issuer                      string `json:"issuer"`
	AuthorizationEndpoint       string `json:"authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	RegistrationEndpoint        string `json:"registration_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	RevocationEndpoint          string `json:"revocation_endpoint"`
}

// oauthCacheEntry is one issuer's worth of cached OAuth state: the
// dynamically registered client id (so registration need not repeat every
// run) and the current token pair.
type oauthCacheEntry struct {
	ClientID     string    `yaml:"client_id"`
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	ExpiresAt    time.Time `yaml:"expires_at"`
}

// oauthTokenResponse is the RFC 6749 section 5.1 access token response
// body, as returned by the token endpoint for every grant type.
type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// oauthTokenError represents a failed token endpoint response: {"error":
// ..., "error_description": ...} with a 4xx status. Its Code is examined
// by the device polling loop to distinguish authorization_pending and
// slow_down (keep polling) from every other outcome (give up).
type oauthTokenError struct {
	Code        string
	Description string
}

func (e *oauthTokenError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth: %s: %s", e.Code, e.Description)
	}
	return "oauth: " + e.Code
}

// OAuthClient drives the CLI side of the OAuth flows against a single
// authorisation server, and caches the resulting tokens on disk across
// runs. The zero value is not ready to use: BaseURL must be set, and the
// exported fields below may be overridden before the first call; each is
// defaulted lazily otherwise.
type OAuthClient struct {
	// BaseURL is the server's origin, with no path (e.g.
	// "http://localhost:8080").
	BaseURL string
	// HTTP is the client used for every request. Defaults to
	// http.DefaultClient.
	HTTP *http.Client
	// CachePath is where the token cache is read from and written to.
	// Defaults to a file named oauth-tokens.yaml beside the CLI's
	// preferences file.
	CachePath string
	// NoBrowser forces the device authorisation flow even when a
	// browser could plausibly be opened.
	NoBrowser bool
	// OpenURL opens a URL in the user's default browser. Defaults to a
	// platform-specific opener. A failure here (rather than a failure of
	// the flow it starts) causes Login to fall back to the device flow.
	OpenURL func(string) error
	// Prompt prints a message to the user, used for the device flow's
	// instructions. A nil Prompt is silently skipped.
	Prompt func(msg string)
	// Now returns the current time, overridable in tests. Defaults to
	// time.Now.
	Now func() time.Time

	mu      sync.Mutex
	meta    *oauthMetadata
	entry   oauthCacheEntry
	subject string
	// sleep is called between device flow polls. Overridable in tests to
	// avoid a real wait; defaults to time.Sleep.
	sleep func(time.Duration)
}

// DiscoverOAuth fetches and parses the RFC 8414 authorisation server
// metadata document at baseURL's well-known path. It returns ErrNoOAuth
// when the server responds 404, which is the expected response from a
// server with OAuth disabled.
func DiscoverOAuth(ctx context.Context, httpc *http.Client, baseURL string) (*oauthMetadata, error) {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	u := strings.TrimSuffix(baseURL, "/") + "/.well-known/oauth-authorization-server"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth metadata request: %w", err)
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth metadata request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoOAuth
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth metadata: unexpected status %d", resp.StatusCode)
	}

	var m oauthMetadata
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("oauth metadata: decode: %w", err)
	}
	return &m, nil
}

// httpClient returns c.HTTP, defaulting to http.DefaultClient.
func (c *OAuthClient) httpClient() *http.Client {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	return c.HTTP
}

// now returns c.Now(), defaulting to time.Now.
func (c *OAuthClient) now() time.Time {
	if c.Now == nil {
		c.Now = time.Now
	}
	return c.Now()
}

// cachePath returns c.CachePath, defaulting to a file beside the CLI's
// preferences file.
func (c *OAuthClient) cachePath() string {
	if c.CachePath != "" {
		return c.CachePath
	}
	return filepath.Join(filepath.Dir(GetPreferencesPath()), "oauth-tokens.yaml")
}

// ensureMeta discovers the authorisation server's metadata if it has not
// been fetched already, so that Login, Token and Logout can each be
// called independently without every caller repeating discovery.
func (c *OAuthClient) ensureMeta(ctx context.Context) error {
	if c.meta != nil {
		return nil
	}
	meta, err := DiscoverOAuth(ctx, c.httpClient(), c.BaseURL)
	if err != nil {
		return err
	}
	c.meta = meta
	return nil
}

// loadOAuthCache reads the token cache at path, returning an empty map
// (not an error) when the file does not yet exist.
func loadOAuthCache(path string) (map[string]oauthCacheEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]oauthCacheEntry{}, nil
		}
		return nil, fmt.Errorf("read oauth cache: %w", err)
	}
	cache := map[string]oauthCacheEntry{}
	if err := yaml.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse oauth cache: %w", err)
	}
	if cache == nil {
		cache = map[string]oauthCacheEntry{}
	}
	return cache, nil
}

// saveOAuthCache writes cache to path, creating its parent directory
// (mode 0700) if necessary and writing the file itself with mode 0600,
// since it holds bearer tokens.
func saveOAuthCache(path string, cache map[string]oauthCacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create oauth cache directory: %w", err)
	}
	data, err := yaml.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshal oauth cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write oauth cache: %w", err)
	}
	return nil
}

// persistLocked writes c.entry into the on-disk cache under the current
// issuer. The caller must hold c.mu.
func (c *OAuthClient) persistLocked() error {
	cache, err := loadOAuthCache(c.cachePath())
	if err != nil {
		return err
	}
	cache[c.meta.Issuer] = c.entry
	return saveOAuthCache(c.cachePath(), cache)
}

// clearCacheLocked removes the current issuer's entry from the on-disk
// cache, if present. The caller must hold c.mu.
func (c *OAuthClient) clearCacheLocked() error {
	cache, err := loadOAuthCache(c.cachePath())
	if err != nil {
		return err
	}
	if c.meta != nil {
		delete(cache, c.meta.Issuer)
	}
	return saveOAuthCache(c.cachePath(), cache)
}

// Login ensures the client holds a current access token, using (in
// order) a still-valid cached token, a refresh of an expired one, or an
// interactive login (loopback or device, per shouldUseDeviceFlow) when
// neither is available or the refresh fails.
func (c *OAuthClient) Login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureMeta(ctx); err != nil {
		return err
	}

	cache, err := loadOAuthCache(c.cachePath())
	if err != nil {
		return err
	}
	if entry, ok := cache[c.meta.Issuer]; ok {
		c.entry = entry
	}

	if c.entry.AccessToken != "" && c.entry.ExpiresAt.After(c.now().Add(oauthRefreshSkew)) {
		return nil
	}
	if c.entry.RefreshToken != "" {
		if err := c.refreshLocked(ctx); err == nil {
			return nil
		}
		// Refresh failed (revoked, expired beyond the server's own
		// grace, or the server forgot it): fall through to a fresh
		// interactive login, keeping only the client id we already
		// registered.
		c.entry = oauthCacheEntry{ClientID: c.entry.ClientID}
	}

	clientID := c.entry.ClientID
	if clientID == "" {
		clientID, err = c.register(ctx)
		if err != nil {
			return fmt.Errorf("oauth client registration: %w", err)
		}
		c.entry.ClientID = clientID
	}

	if c.NoBrowser || c.shouldUseDeviceFlow() {
		return c.loginDevice(ctx, clientID)
	}

	err = c.loginLoopback(ctx, clientID)
	if errors.Is(err, errOpenBrowserFailed) {
		return c.loginDevice(ctx, clientID)
	}
	return err
}

// shouldUseDeviceFlow reports whether the environment looks unable to
// display a browser: specifically, a Linux session with neither X11 nor
// Wayland running. Other platforms are assumed capable, and NoBrowser is
// checked separately by the caller.
func (c *OAuthClient) shouldUseDeviceFlow() bool {
	return runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

// register performs RFC 7591 dynamic client registration for a public
// (no client secret) client using the loopback redirect URI. The server
// matches a loopback redirect URI on any port at authorisation and token
// time, so registering the bare "http://127.0.0.1/callback" covers every
// port the CLI's own listener ends up bound to.
func (c *OAuthClient) register(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]any{
		"redirect_uris":              []string{"http://127.0.0.1" + oauthLoopbackRedirectPath},
		"client_name":                "pgEdge NLA CLI",
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token", oauthDeviceGrantType},
	})
	if err != nil {
		return "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, oauthTokenRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.meta.RegistrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode registration response: %w", err)
	}
	if out.ClientID == "" {
		return "", errors.New("registration response carried no client_id")
	}
	return out.ClientID, nil
}

// loginLoopback runs the authorisation code grant with PKCE via a
// one-shot local HTTP listener: it opens the authorisation URL in a
// browser, waits for the resulting redirect to reach the listener, and
// exchanges the code it carries for tokens.
func (c *OAuthClient) loginLoopback(ctx context.Context, clientID string) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("open loopback listener: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d%s", port, oauthLoopbackRedirectPath)

	verifier, challenge, err := generatePKCE()
	if err != nil {
		ln.Close()
		return err
	}
	state, err := randomURLSafeString(16)
	if err != nil {
		ln.Close()
		return err
	}

	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(oauthLoopbackRedirectPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("error") != "":
			desc := q.Get("error_description")
			fmt.Fprintf(w, "<html><body>Login failed: %s. You can return to the terminal.</body></html>", htmlEscape(q.Get("error")))
			resultCh <- result{err: fmt.Errorf("authorisation server denied the request: %s %s", q.Get("error"), desc)}
		case q.Get("state") != state:
			fmt.Fprint(w, "<html><body>Login failed: invalid state. You can return to the terminal.</body></html>")
			resultCh <- result{err: errors.New("authorisation response carried an unexpected state parameter")}
		case q.Get("code") == "":
			fmt.Fprint(w, "<html><body>Login failed: no authorisation code received. You can return to the terminal.</body></html>")
			resultCh <- result{err: errors.New("authorisation response carried no code")}
		default:
			fmt.Fprint(w, "<html><body>Login complete. You can return to the terminal.</body></html>")
			resultCh <- result{code: q.Get("code")}
		}
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	authURL := c.buildAuthorizeURL(clientID, redirectURI, state, challenge)

	openURL := c.OpenURL
	if openURL == nil {
		openURL = openBrowser
	}
	if err := openURL(authURL); err != nil {
		return fmt.Errorf("%w: %v", errOpenBrowserFailed, err)
	}

	select {
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		return c.exchangeAuthCode(ctx, clientID, redirectURI, res.code, verifier)
	case <-time.After(oauthLoginTimeout):
		return errors.New("timed out waiting for the browser login to complete")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// buildAuthorizeURL builds the authorisation endpoint URL for a fresh
// authorisation code request.
func (c *OAuthClient) buildAuthorizeURL(clientID, redirectURI, state, challenge string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"scope":                 {"mcp"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return c.meta.AuthorizationEndpoint + "?" + q.Encode()
}

// exchangeAuthCode redeems an authorisation code for a token pair via the
// authorization_code grant, and persists the result.
func (c *OAuthClient) exchangeAuthCode(ctx context.Context, clientID, redirectURI, code, verifier string) error {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	return c.requestTokenLocked(ctx, form, clientID)
}

// loginDevice runs the device authorisation grant (RFC 8628): it
// requests a device and user code, prints instructions for the user via
// Prompt, and polls the token endpoint until the user approves (or denies
// or ignores) the request on another device.
func (c *OAuthClient) loginDevice(ctx context.Context, clientID string) error {
	form := url.Values{
		"client_id": {clientID},
		"scope":     {"mcp"},
	}

	reqCtx, cancel := context.WithTimeout(ctx, oauthTokenRequestTimeout)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.meta.DeviceAuthorizationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		cancel()
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient().Do(req)
	cancel()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device authorisation request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var dr struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int64  `json:"expires_in"`
		Interval                int64  `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return fmt.Errorf("decode device authorisation response: %w", err)
	}

	if c.Prompt != nil {
		c.Prompt(fmt.Sprintf("Open %s in a browser and enter code %s if asked.", dr.VerificationURIComplete, dr.UserCode))
	}

	interval := time.Duration(dr.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := c.now().Add(time.Duration(dr.ExpiresIn) * time.Second)

	sleep := c.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	pollForm := url.Values{
		"grant_type":  {oauthDeviceGrantType},
		"device_code": {dr.DeviceCode},
		"client_id":   {clientID},
	}

	for {
		if c.now().After(deadline) {
			return errors.New("device login expired before it was approved")
		}
		sleep(interval)

		err := c.requestTokenLocked(ctx, pollForm, clientID)
		if err == nil {
			return nil
		}
		var te *oauthTokenError
		if errors.As(err, &te) {
			switch te.Code {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				continue
			}
		}
		return err
	}
}

// refreshLocked exchanges the current refresh token for a fresh token
// pair. The caller must hold c.mu, and c.entry.RefreshToken and
// c.entry.ClientID must already be populated.
func (c *OAuthClient) refreshLocked(ctx context.Context) error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.entry.RefreshToken},
		"client_id":     {c.entry.ClientID},
	}
	return c.requestTokenLocked(ctx, form, c.entry.ClientID)
}

// requestTokenLocked posts form to the token endpoint, and on success
// updates c.entry and persists it to the cache. The caller must hold
// c.mu. A refresh grant that omits refresh_token in its response (the
// server is not required to rotate it) keeps the previous one.
func (c *OAuthClient) requestTokenLocked(ctx context.Context, form url.Values, clientID string) error {
	reqCtx, cancel := context.WithTimeout(ctx, oauthTokenRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var oe struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oe)
		if oe.Error == "" {
			oe.Error = "server_error"
		}
		return &oauthTokenError{Code: oe.Error, Description: oe.ErrorDescription}
	}

	var tr oauthTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}

	c.entry.ClientID = clientID
	c.entry.AccessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		c.entry.RefreshToken = tr.RefreshToken
	}
	c.entry.ExpiresAt = c.now().Add(time.Duration(tr.ExpiresIn) * time.Second)

	return c.persistLocked()
}

// Token implements TokenSource: it returns the current access token,
// transparently refreshing it first when it is within oauthRefreshSkew of
// expiring. A refresh failure clears the cached entry and returns "" so
// the caller's request fails with 401, which the chat client reports;
// Login is retried on the next connection attempt.
func (c *OAuthClient) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), oauthTokenRequestTimeout)
	defer cancel()

	if err := c.ensureMeta(ctx); err != nil {
		return ""
	}

	if c.entry.AccessToken == "" && c.entry.RefreshToken == "" {
		cache, err := loadOAuthCache(c.cachePath())
		if err == nil {
			if entry, ok := cache[c.meta.Issuer]; ok {
				c.entry = entry
			}
		}
	}

	if c.entry.AccessToken == "" {
		return ""
	}
	if c.entry.ExpiresAt.After(c.now().Add(oauthRefreshSkew)) {
		return c.entry.AccessToken
	}
	if c.entry.RefreshToken == "" {
		// No way to refresh: hand back what we have and let the server
		// reject it if it has expired.
		return c.entry.AccessToken
	}

	if err := c.refreshLocked(ctx); err != nil {
		c.entry = oauthCacheEntry{}
		_ = c.clearCacheLocked()
		return ""
	}
	return c.entry.AccessToken
}

// Logout revokes the cached refresh token, if any, and removes the
// issuer's entry from the on-disk cache regardless of whether revocation
// succeeded, so a failed request against an unreachable server does not
// leave the CLI unable to log out locally.
func (c *OAuthClient) Logout(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureMeta(ctx); err != nil {
		return err
	}

	cache, err := loadOAuthCache(c.cachePath())
	if err != nil {
		return err
	}
	entry, ok := cache[c.meta.Issuer]
	if ok && entry.RefreshToken != "" && c.meta.RevocationEndpoint != "" {
		form := url.Values{"token": {entry.RefreshToken}}
		reqCtx, cancel := context.WithTimeout(ctx, oauthTokenRequestTimeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.meta.RevocationEndpoint, strings.NewReader(form.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if resp, doErr := c.httpClient().Do(req); doErr == nil {
				resp.Body.Close()
			}
		}
		cancel()
	}

	delete(cache, c.meta.Issuer)
	c.entry = oauthCacheEntry{}
	c.subject = ""
	return saveOAuthCache(c.cachePath(), cache)
}

// Subject returns the authenticated user's identifier, fetched from
// <BaseURL>/api/user/info with the current access token and cached for
// the life of the client. It returns "" if not logged in or the lookup
// fails.
func (c *OAuthClient) Subject() string {
	c.mu.Lock()
	if c.subject != "" {
		subject := c.subject
		c.mu.Unlock()
		return subject
	}
	token := c.entry.AccessToken
	c.mu.Unlock()

	if token == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), oauthTokenRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(c.BaseURL, "/")+"/api/user/info", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var info struct {
		Authenticated bool   `json:"authenticated"`
		Username      string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || !info.Authenticated {
		return ""
	}

	c.mu.Lock()
	c.subject = info.Username
	c.mu.Unlock()
	return info.Username
}

// generatePKCE returns a fresh RFC 7636 code verifier (a 43-character
// unpadded base64url string, drawn from 32 random bytes) and its S256
// challenge.
func generatePKCE() (verifier, challenge string, err error) {
	verifier, err = randomURLSafeString(32)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// randomURLSafeString returns n random bytes, base64url-encoded without
// padding.
func randomURLSafeString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random string: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// htmlEscape escapes the handful of characters that matter when echoing
// a server-supplied error code into the loopback callback's HTML page.
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
