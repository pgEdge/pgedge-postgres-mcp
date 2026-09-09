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
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pgedge-postgres-mcp/internal/config"
)

//go:embed templates/login.html
var defaultTemplate string

//go:embed templates/logo-light.png
var defaultLogo []byte

// Branding holds the values used to render the login page, derived from
// config.LoginPageConfig.
type Branding struct {
	Title             string
	Subtitle          string
	MessageParagraphs []string
	Footer            string
	LogoURL           string // "/oauth/static/logo"
	PrimaryColour     template.CSS
	SecondaryColour   template.CSS
}

// AuthorizeParams carries the OAuth 2.0 authorisation request parameters
// through the login form as hidden fields, so they survive the round trip
// to the credential submission handler.
type AuthorizeParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// LoginPageData is the data passed to the login page template.
type LoginPageData struct {
	Branding     Branding
	Error        string
	CSRFToken    string
	Client       string
	OAuth        AuthorizeParams
	UserCode     string
	IsDeviceFlow bool
	Page         string // "login", "device", "done"
}

// loginPage renders the branded OAuth login page and serves its logo.
type loginPage struct {
	tmpl     *template.Template
	branding Branding
	logo     []byte
	logoType string
}

// newLoginPage builds a loginPage from the given configuration, parsing the
// embedded template (or a custom TemplateFile, if configured) and loading
// the logo (embedded by default, or a custom LogoFile).
func newLoginPage(cfg config.LoginPageConfig) (*loginPage, error) {
	var (
		tmpl *template.Template
		err  error
	)
	if cfg.TemplateFile != "" {
		tmpl, err = template.New(filepath.Base(cfg.TemplateFile)).ParseFiles(cfg.TemplateFile)
		if err != nil {
			return nil, fmt.Errorf("parsing login template file %q: %w", cfg.TemplateFile, err)
		}
	} else {
		tmpl, err = template.New("login.html").Parse(defaultTemplate)
		if err != nil {
			return nil, fmt.Errorf("parsing default login template: %w", err)
		}
	}

	logo := defaultLogo
	logoType := "image/png"
	if cfg.LogoFile != "" {
		data, err := os.ReadFile(cfg.LogoFile)
		if err != nil {
			return nil, fmt.Errorf("reading login logo file %q: %w", cfg.LogoFile, err)
		}
		logo = data
		logoType = detectContentType(cfg.LogoFile, data)
	}

	return &loginPage{
		tmpl: tmpl,
		branding: Branding{
			Title:             cfg.Title,
			Subtitle:          cfg.Subtitle,
			MessageParagraphs: splitParagraphs(cfg.Message),
			Footer:            cfg.Footer,
			LogoURL:           LogoPath,
			PrimaryColour:     template.CSS(cfg.PrimaryColour),
			SecondaryColour:   template.CSS(cfg.SecondaryColour),
		},
		logo:     logo,
		logoType: logoType,
	}, nil
}

// detectContentType returns the MIME type for a logo file, based on its
// extension where recognised, falling back to content sniffing.
func detectContentType(path string, data []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return http.DetectContentType(data)
	}
}

// Render executes the login page template into a buffer and, only once
// that succeeds, writes the security headers, status and body. Rendering
// via a buffer first ensures a template error yields a 500 response rather
// than a half-written page.
func (p *loginPage) Render(w http.ResponseWriter, status int, data LoginPageData) error {
	data.Branding = p.branding
	var buf bytes.Buffer
	if err := p.tmpl.ExecuteTemplate(&buf, "page", data); err != nil {
		return fmt.Errorf("rendering login page: %w", err)
	}

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; form-action 'self'")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// ServeLogo writes the configured logo image, embedded by default or a
// custom LogoFile if configured, with a cacheable response.
func (p *loginPage) ServeLogo(w http.ResponseWriter, _ *http.Request) {
	h := w.Header()
	h.Set("Content-Type", p.logoType)
	h.Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(p.logo)
}

// splitParagraphs splits s on blank lines, trims surrounding whitespace
// from each paragraph, and drops any that are empty as a result.
func splitParagraphs(s string) []string {
	if s == "" {
		return nil
	}
	var (
		paragraphs []string
		lines      []string
	)
	flush := func() {
		if len(lines) == 0 {
			return
		}
		paragraph := strings.TrimSpace(strings.Join(lines, "\n"))
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
		lines = nil
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		lines = append(lines, strings.TrimRight(line, " \t\r"))
	}
	flush()
	return paragraphs
}
