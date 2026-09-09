/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package oauth

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

func renderToString(t *testing.T, p *loginPage, d LoginPageData) string {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := p.Render(rec, 200, d); err != nil {
		t.Fatal(err)
	}
	return rec.Body.String()
}

func TestDefaultTemplateRendersBrandingAndHiddenFields(t *testing.T) {
	p, err := newLoginPage(config.LoginPageConfig{Title: "Sign in", Subtitle: "Welcome", Footer: "pgEdge",
		Message: "Authorised users only.\n\nActivity is logged.", PrimaryColour: "#15AABF", SecondaryColour: "#0C8599"})
	if err != nil {
		t.Fatal(err)
	}
	out := renderToString(t, p, LoginPageData{Page: "login", CSRFToken: "tok", Client: "Claude",
		OAuth: AuthorizeParams{ResponseType: "code", ClientID: "c1", RedirectURI: "https://claude.ai/cb", State: "st", CodeChallenge: "ch", CodeChallengeMethod: "S256", Scope: "mcp"}})
	for _, want := range []string{"<title>Sign in</title>", "Welcome", "<p>Authorised users only.</p>", "<p>Activity is logged.</p>", "pgEdge",
		`name="csrf_token" value="tok"`, `name="state" value="st"`, `name="code_challenge" value="ch"`, `name="redirect_uri" value="https://claude.ai/cb"`,
		"#15AABF", "/oauth/static/logo", "Claude"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestMessageIsEscaped(t *testing.T) {
	p, _ := newLoginPage(config.LoginPageConfig{Message: "<script>alert(1)</script>", PrimaryColour: "#000", SecondaryColour: "#000"})
	out := renderToString(t, p, LoginPageData{Page: "login"})
	if strings.Contains(out, "<script>") {
		t.Fatal("markup not escaped")
	}
}

func TestDevicePageHasUserCodeField(t *testing.T) {
	p, _ := newLoginPage(config.LoginPageConfig{PrimaryColour: "#000", SecondaryColour: "#000"})
	out := renderToString(t, p, LoginPageData{Page: "device", IsDeviceFlow: true, UserCode: "ABCD-EFGH"})
	if !strings.Contains(out, `name="user_code"`) || !strings.Contains(out, "ABCD-EFGH") {
		t.Fatal(out)
	}
}

func TestCustomTemplateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "login.html")
	_ = os.WriteFile(path, []byte(`{{define "page"}}CUSTOM {{.Branding.Title}} {{.CSRFToken}}{{end}}`), 0600)
	p, err := newLoginPage(config.LoginPageConfig{Title: "T", TemplateFile: path, PrimaryColour: "#000", SecondaryColour: "#000"})
	if err != nil {
		t.Fatal(err)
	}
	if out := renderToString(t, p, LoginPageData{Page: "login", CSRFToken: "x"}); out != "CUSTOM T x" {
		t.Fatalf("%q", out)
	}
}

func TestCustomLogoServed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logo.jpg")
	_ = os.WriteFile(path, []byte("\xff\xd8\xff\xe0not really a jpeg"), 0600)
	p, err := newLoginPage(config.LoginPageConfig{LogoFile: path, PrimaryColour: "#000", SecondaryColour: "#000"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.ServeLogo(rec, httptest.NewRequest("GET", LogoPath, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal(rec.Code, rec.Header())
	}
}

// TestSVGLogoRejected covers the finding that an SVG logo is refused:
// it is an active content type, served from the same origin as the
// login page, so it is not an acceptable image format here.
func TestSVGLogoRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logo.svg")
	_ = os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0600)
	if _, err := newLoginPage(config.LoginPageConfig{LogoFile: path, PrimaryColour: "#000", SecondaryColour: "#000"}); err == nil {
		t.Fatal("expected an SVG logo to be rejected")
	}
}

func TestUnknownLogoExtensionRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logo.bmp")
	_ = os.WriteFile(path, []byte("BM"), 0600)
	if _, err := newLoginPage(config.LoginPageConfig{LogoFile: path, PrimaryColour: "#000", SecondaryColour: "#000"}); err == nil {
		t.Fatal("expected an unknown logo extension to be rejected")
	}
}

// TestTemplateWithoutPageDefinitionRejected covers the finding that a
// custom template which never defines "page" must fail at startup
// rather than at the first login attempt.
func TestTemplateWithoutPageDefinitionRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "login.html")
	_ = os.WriteFile(path, []byte(`no page definition here`), 0600)
	if _, err := newLoginPage(config.LoginPageConfig{TemplateFile: path, PrimaryColour: "#000", SecondaryColour: "#000"}); err == nil {
		t.Fatal("expected a template without a page definition to be rejected")
	}
}

// TestLoginPageSecurityHeaders covers the login page CSP and header
// finding: form-action is gone (Chromium applies it to the post-submit
// redirect chain), base-uri and frame-ancestors are locked down, and
// the sniffing and referrer headers are set.
func TestLoginPageSecurityHeaders(t *testing.T) {
	p, _ := newLoginPage(config.LoginPageConfig{PrimaryColour: "#000", SecondaryColour: "#000"})
	rec := httptest.NewRecorder()
	if err := p.Render(rec, 200, LoginPageData{Page: "login"}); err != nil {
		t.Fatal(err)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "form-action") {
		t.Errorf("form-action must not be set: %q", csp)
	}
	for _, want := range []string{"base-uri 'none'", "frame-ancestors 'none'", "default-src 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q missing %q", csp, want)
		}
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("missing Referrer-Policy")
	}
}

func TestServeLogoSecurityHeaders(t *testing.T) {
	p, _ := newLoginPage(config.LoginPageConfig{PrimaryColour: "#000", SecondaryColour: "#000"})
	rec := httptest.NewRecorder()
	p.ServeLogo(rec, httptest.NewRequest("GET", LogoPath, nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'; sandbox" {
		t.Errorf("CSP = %q", got)
	}
}

// TestErrorPageHasNoForm covers the error variant of the template,
// rendered when the client or redirect URI cannot be trusted.
func TestErrorPageHasNoForm(t *testing.T) {
	p, _ := newLoginPage(config.LoginPageConfig{PrimaryColour: "#000", SecondaryColour: "#000"})
	out := renderToString(t, p, LoginPageData{Page: "error", Error: "Invalid client or redirect URI"})
	if strings.Contains(out, "<form") || strings.Contains(out, `name="username"`) {
		t.Errorf("error page must not contain a form:\n%s", out)
	}
	if !strings.Contains(out, "Invalid client or redirect URI") {
		t.Errorf("error page missing the message:\n%s", out)
	}
}

// TestRenderErrorWritesPlain500 covers the finding that a failed render
// must produce a 500, not an empty 200.
func TestRenderErrorWritesPlain500(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "login.html")
	_ = os.WriteFile(path, []byte(`{{define "page"}}{{.Branding.Title.Nope}}{{end}}`), 0600)
	p, err := newLoginPage(config.LoginPageConfig{TemplateFile: path, PrimaryColour: "#000", SecondaryColour: "#000"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := p.Render(rec, 200, LoginPageData{Page: "login"}); err == nil {
		t.Fatal("expected a render error")
	}
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// TestFocusRingUsesOutline covers the focus ring finding: appending an
// alpha suffix to the configured colour is wrong for #rgb and
// #rrggbbaa values, so the template uses an outline instead.
func TestFocusRingUsesOutline(t *testing.T) {
	if strings.Contains(defaultTemplate, "PrimaryColour}}33") {
		t.Error("template still appends an alpha suffix to the primary colour")
	}
	if !strings.Contains(defaultTemplate, "outline: 2px solid") {
		t.Error("template does not use an outline for the focus ring")
	}
}

func TestEmbeddedLogoServedByDefault(t *testing.T) {
	p, _ := newLoginPage(config.LoginPageConfig{PrimaryColour: "#000", SecondaryColour: "#000"})
	rec := httptest.NewRecorder()
	p.ServeLogo(rec, httptest.NewRequest("GET", LogoPath, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Body.Len() == 0 {
		t.Fatal(rec.Code)
	}
}

func TestSplitParagraphs(t *testing.T) {
	got := splitParagraphs("a\nb\n\n\n c \n\n")
	if !slices.Equal(got, []string{"a\nb", "c"}) {
		t.Fatalf("%q", got)
	}
}
