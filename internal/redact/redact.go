/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package redact removes credentials from text that is about to leave the
// process, whether to an HTTP client, a trace file or a terminal.
//
// It exists because an LLM provider's error body is not ours to control. When
// authentication fails, providers characteristically quote the key they were
// given back at the caller: OpenAI's 401 message names the key it rejected, in
// a partially masked form that still discloses the leading characters. The
// shared LLM library relays a provider's message verbatim into the error it
// returns, and that error is written into an HTTP response body and into trace
// entries, so the fragment travels with it.
//
// The right place to stop that is the library, so that every consumer benefits
// and the credential never reaches an error value in the first place. This
// package is the layer beneath that: it assumes a leak has already happened
// and scrubs the text on its way out.
//
// Being a filter, it is inherently incomplete. It recognises the shapes of the
// credentials this project handles and the values it has been told about; a
// provider that invents a new format, or quotes a key in a way that defeats
// the patterns here, will not be caught. Treat it as a safety net rather than
// as the guarantee, and keep secrets out of error values upstream.
package redact

import (
	"regexp"
	"strings"
	"sync"
)

// Placeholder replaces any credential that is found. It matches the wording
// already used by the tracer for redacted parameters.
const Placeholder = "[REDACTED]"

// minRegisterLength is the shortest secret worth registering. Anything shorter
// risks matching ordinary words and doing more damage to the text than the
// disclosure would.
const minRegisterLength = 8

// minFragmentLength is the shortest leading fragment of a registered secret
// that is replaced on its own. Providers quote a truncated key rather than the
// whole thing, so the fragment matters more than the full value, but a very
// short fragment carries little information and matches too readily.
const minFragmentLength = 12

// keyPatterns match the credential formats this project handles.
//
// The character classes include '*' and '.' so that a partially masked key is
// consumed in full rather than leaving its tail behind: a message quoting
// "sk-proj-abcd****wxyz" must not be reduced to "[REDACTED]****wxyz".
var keyPatterns = []*regexp.Regexp{
	// OpenAI and Anthropic, covering the sk-proj- and sk-ant- variants.
	regexp.MustCompile(`sk-[A-Za-z0-9_.*-]{8,}`),
	// Google AI Studio, used for Gemini.
	regexp.MustCompile(`AIza[A-Za-z0-9_.*-]{10,}`),
	// Voyage, used for embeddings.
	regexp.MustCompile(`pa-[A-Za-z0-9_.*-]{8,}`),
	// An Authorization header, in case one is ever quoted back.
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._*-]{8,}`),
}

var (
	mu      sync.RWMutex
	secrets []string
)

// Register records secrets that must never appear in outgoing text: the
// provider API keys this server was configured with. Registering a value
// allows an exact match, which catches a key whose format the patterns above
// do not recognise.
//
// Call it during startup, before the server begins serving. Values shorter
// than minRegisterLength, and duplicates, are ignored.
func Register(values ...string) {
	mu.Lock()
	defer mu.Unlock()

	for _, v := range values {
		v = strings.TrimSpace(v)
		if len(v) < minRegisterLength {
			continue
		}
		if slicesContains(secrets, v) {
			continue
		}
		secrets = append(secrets, v)
	}
}

// Reset discards every registered secret. It exists for tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	secrets = nil
}

// String returns s with every registered secret, every leading fragment of a
// registered secret, and everything shaped like a provider API key replaced by
// Placeholder.
func String(s string) string {
	if s == "" {
		return s
	}

	s = replaceRegistered(s)

	for _, pattern := range keyPatterns {
		s = pattern.ReplaceAllStringFunc(s, func(match string) string {
			// A trailing full stop is far more likely to end the sentence
			// than the credential, so leave it in place.
			if trimmed := strings.TrimRight(match, "."); trimmed != match {
				return Placeholder + match[len(trimmed):]
			}
			return Placeholder
		})
	}

	return s
}

// Bytes is String for a byte slice. The input is never modified in place.
func Bytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	out := String(string(b))
	if out == string(b) {
		return b
	}
	return []byte(out)
}

// Error returns err.Error() with credentials removed, and an empty string for
// a nil error. Use it wherever an error is about to be shown to somebody or
// written to a file.
func Error(err error) string {
	if err == nil {
		return ""
	}
	return String(err.Error())
}

// ErrorValue returns an error whose message has been redacted, for the places
// that must pass an error along rather than a string. It returns nil for a nil
// error, and the original error when there was nothing to remove.
//
// The result keeps the original error in its chain, so errors.Is and errors.As
// continue to work against whatever sentinel the library wrapped; only the
// message the error presents is cleaned.
func ErrorValue(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	redacted := String(message)
	if redacted == message {
		return err
	}
	return &redactedError{message: redacted, cause: err}
}

// redactedError presents a cleaned message whilst preserving the error chain.
type redactedError struct {
	message string
	cause   error
}

func (e *redactedError) Error() string { return e.message }

func (e *redactedError) Unwrap() error { return e.cause }

// replaceRegistered removes registered secrets, whole or truncated.
//
// The truncated case is the one that matters in practice, because a provider
// quotes the start of the key rather than all of it, so the longest leading
// fragment present in the text is replaced. Only the longest match is needed:
// removing it removes every shorter prefix along with it.
func replaceRegistered(s string) string {
	mu.RLock()
	defer mu.RUnlock()

	for _, secret := range secrets {
		if strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, Placeholder)
			continue
		}
		for n := len(secret) - 1; n >= minFragmentLength; n-- {
			fragment := secret[:n]
			if strings.Contains(s, fragment) {
				s = strings.ReplaceAll(s, fragment, Placeholder)
				break
			}
		}
	}

	return s
}

// slicesContains reports whether v is present in list.
func slicesContains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
