/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package redact

import (
	"errors"
	"strings"
	"testing"
)

// Every key below is invented for the test. The formats mimic the real ones so
// that the patterns are exercised, but the values are not credentials.
const (
	fakeOpenAIKey    = "sk-proj-AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJ"
	fakeAnthropicKey = "sk-ant-api03-KKKKLLLLMMMMNNNNOOOOPPPPQQQQRRRR"
	fakeGeminiKey    = "AIzaSSSSTTTTUUUUVVVVWWWWXXXXYYYYZZZZ012"
	fakeVoyageKey    = "pa-0123456789abcdefghijklmnopqrstuvwxyz"
)

func TestStringRedactsKeyShapes(t *testing.T) {
	Reset()

	tests := []struct {
		name  string
		input string
		// mustNotContain is the fragment that must be gone afterwards.
		mustNotContain string
	}{
		{
			// This is the message shape that prompted the work: a provider
			// quoting back the key it rejected.
			name:           "OpenAI style authentication failure",
			input:          "openai (401): Incorrect API key provided: " + fakeOpenAIKey + ". You can find your API key at https://platform.openai.com/account/api-keys.",
			mustNotContain: "AAAABBBB",
		},
		{
			name:           "partially masked key keeps no tail",
			input:          "Incorrect API key provided: sk-proj-AAAA****JJJJ.",
			mustNotContain: "JJJJ",
		},
		{
			name:           "Anthropic key",
			input:          "anthropic (401): invalid x-api-key " + fakeAnthropicKey,
			mustNotContain: "KKKKLLLL",
		},
		{
			name:           "Gemini key",
			input:          "gemini (400): API key not valid: " + fakeGeminiKey,
			mustNotContain: "SSSSTTTT",
		},
		{
			name:           "Voyage key",
			input:          "voyage (401): bad key " + fakeVoyageKey,
			mustNotContain: "0123456789abcdef",
		},
		{
			name:           "bearer token",
			input:          "upstream rejected Authorization: Bearer abcdefghijklmnop",
			mustNotContain: "abcdefghijklmnop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := String(tt.input)

			if strings.Contains(got, tt.mustNotContain) {
				t.Errorf("redacted text still contains %q: %s", tt.mustNotContain, got)
			}
			if !strings.Contains(got, Placeholder) {
				t.Errorf("expected %s in the result, got: %s", Placeholder, got)
			}
		})
	}
}

func TestStringPreservesSurroundingText(t *testing.T) {
	Reset()

	got := String("openai (401): Incorrect API key provided: " + fakeOpenAIKey + ". Check your settings.")

	// The message must stay useful: whoever reads it still needs to know what
	// went wrong and with which provider.
	for _, want := range []string{"openai", "401", "Incorrect API key provided", "Check your settings."} {
		if !strings.Contains(got, want) {
			t.Errorf("redaction removed useful context %q: %s", want, got)
		}
	}

	// A full stop that ends the sentence belongs to the sentence.
	if !strings.Contains(got, Placeholder+".") {
		t.Errorf("expected the sentence's full stop to survive: %s", got)
	}
}

func TestStringLeavesOrdinaryTextAlone(t *testing.T) {
	Reset()

	unchanged := []string{
		"",
		"connection refused",
		"pq: relation \"users\" does not exist",
		"openai (429): rate limit exceeded, retry after 20s",
		"SELECT * FROM sk_units WHERE id = 5",
		"task-list is empty",
	}

	for _, input := range unchanged {
		if got := String(input); got != input {
			t.Errorf("String(%q) = %q, want it unchanged", input, got)
		}
	}
}

func TestRegisteredSecretIsRedactedWholeAndTruncated(t *testing.T) {
	Reset()
	defer Reset()

	// A key whose format none of the patterns recognise, which is exactly why
	// registration exists.
	const key = "custom-format-9f3b1d7c2e4a6b8d0f2a4c6e"
	Register(key)

	t.Run("whole value", func(t *testing.T) {
		got := String("provider rejected " + key)
		if strings.Contains(got, key) {
			t.Errorf("registered secret survived: %s", got)
		}
		if !strings.Contains(got, Placeholder) {
			t.Errorf("expected a placeholder: %s", got)
		}
	})

	t.Run("truncated value", func(t *testing.T) {
		// Providers quote a prefix rather than the whole key, so the prefix
		// has to go too.
		got := String("provider rejected " + key[:20] + "...")
		if strings.Contains(got, key[:20]) {
			t.Errorf("truncated secret survived: %s", got)
		}
	})

	t.Run("fragment too short to be worth matching", func(t *testing.T) {
		// Below the fragment threshold nothing is replaced, which is
		// deliberate: short fragments carry little and match too much.
		short := key[:6]
		if got := String("value " + short); strings.Contains(got, Placeholder) {
			t.Errorf("did not expect a %s-length fragment to be redacted: %s", short, got)
		}
	})
}

func TestRegisterIgnoresShortAndDuplicateValues(t *testing.T) {
	Reset()
	defer Reset()

	Register("", "abc", "   ")
	if got := String("abc"); got != "abc" {
		t.Errorf("a short value should not have been registered, got %q", got)
	}

	Register(fakeOpenAIKey, fakeOpenAIKey)

	mu.RLock()
	count := len(secrets)
	mu.RUnlock()
	if count != 1 {
		t.Errorf("registered secret count = %d, want 1", count)
	}
}

func TestErrorRedacts(t *testing.T) {
	Reset()

	if got := Error(nil); got != "" {
		t.Errorf("Error(nil) = %q, want empty", got)
	}

	err := errors.New("openai (401): Incorrect API key provided: " + fakeOpenAIKey)
	got := Error(err)
	if strings.Contains(got, "AAAABBBB") {
		t.Errorf("Error() leaked the key: %s", got)
	}
}

func TestBytes(t *testing.T) {
	Reset()

	// An input needing no change is returned as-is.
	in := []byte("nothing to see here")
	if got := Bytes(in); string(got) != string(in) {
		t.Errorf("Bytes() = %q, want %q", got, in)
	}

	in = []byte(`{"error":"openai (401): Incorrect API key provided: ` + fakeOpenAIKey + `"}`)
	got := Bytes(in)
	if strings.Contains(string(got), "AAAABBBB") {
		t.Errorf("Bytes() leaked the key: %s", got)
	}
	if !strings.Contains(string(got), Placeholder) {
		t.Errorf("expected a placeholder: %s", got)
	}

	// The caller's slice must not be modified in place.
	if !strings.Contains(string(in), "AAAABBBB") {
		t.Error("Bytes() modified its input")
	}
}
