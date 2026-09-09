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
	path := filepath.Join(dir, "logo.svg")
	_ = os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0600)
	p, err := newLoginPage(config.LoginPageConfig{LogoFile: path, PrimaryColour: "#000", SecondaryColour: "#000"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.ServeLogo(rec, httptest.NewRequest("GET", LogoPath, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatal(rec.Code, rec.Header())
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
