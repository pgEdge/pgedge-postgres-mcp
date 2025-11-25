/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package compactor

import (
	"testing"
)

func TestNewTokenCounterConfig(t *testing.T) {
	tests := []struct {
		name         string
		counterType  TokenCounterType
		wantChars    float64
		wantOverhead int
	}{
		{
			name:         "OpenAI config",
			counterType:  TokenCounterOpenAI,
			wantChars:    4.0,
			wantOverhead: 4,
		},
		{
			name:         "Anthropic config",
			counterType:  TokenCounterAnthropic,
			wantChars:    3.8,
			wantOverhead: 5,
		},
		{
			name:         "Ollama config",
			counterType:  TokenCounterOllama,
			wantChars:    4.5,
			wantOverhead: 3,
		},
		{
			name:         "Generic config",
			counterType:  TokenCounterGeneric,
			wantChars:    4.0,
			wantOverhead: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewTokenCounterConfig(tt.counterType)
			if config.CharsPerToken != tt.wantChars {
				t.Errorf("CharsPerToken = %v, want %v", config.CharsPerToken, tt.wantChars)
			}
			if config.Overhead != tt.wantOverhead {
				t.Errorf("Overhead = %v, want %v", config.Overhead, tt.wantOverhead)
			}
			if config.Type != tt.counterType {
				t.Errorf("Type = %v, want %v", config.Type, tt.counterType)
			}
		})
	}
}

func TestProviderTokenEstimator_EstimateTokens(t *testing.T) {
	tests := []struct {
		name        string
		counterType TokenCounterType
		text        string
		wantMin     int
		wantMax     int
	}{
		{
			name:        "OpenAI short text",
			counterType: TokenCounterOpenAI,
			text:        "Hello world",
			wantMin:     2,
			wantMax:     10,
		},
		{
			name:        "Anthropic SQL query",
			counterType: TokenCounterAnthropic,
			text:        "SELECT * FROM users WHERE id = 1",
			wantMin:     8,
			wantMax:     20,
		},
		{
			name:        "Ollama JSON content",
			counterType: TokenCounterOllama,
			text:        `{"name": "test", "value": 123}`,
			wantMin:     6,
			wantMax:     15,
		},
		{
			name:        "Generic code content",
			counterType: TokenCounterGeneric,
			text:        "function test() { return 42; }",
			wantMin:     7,
			wantMax:     20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := NewProviderTokenEstimator(tt.counterType)
			tokens := estimator.EstimateTokens(tt.text)
			if tokens < tt.wantMin || tokens > tt.wantMax {
				t.Errorf("EstimateTokens() = %v, want between %v and %v", tokens, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestProviderTokenEstimator_ContentMultiplier(t *testing.T) {
	estimator := NewProviderTokenEstimator(TokenCounterGeneric)

	tests := []struct {
		name           string
		text           string
		expectedApprox float64
	}{
		{
			name:           "SQL content has higher multiplier",
			text:           "SELECT * FROM users",
			expectedApprox: 1.2,
		},
		{
			name:           "JSON content has medium multiplier",
			text:           `{"key": "value"}`,
			expectedApprox: 1.15,
		},
		{
			name:           "Code content has slight multiplier",
			text:           "function test() {}",
			expectedApprox: 1.1,
		},
		{
			name:           "Plain text has no multiplier",
			text:           "This is just plain text",
			expectedApprox: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multiplier := estimator.getContentMultiplier(tt.text)
			if multiplier != tt.expectedApprox {
				t.Errorf("getContentMultiplier() = %v, want %v", multiplier, tt.expectedApprox)
			}
		})
	}
}

func TestProviderTokenEstimator_ProviderAdjustment(t *testing.T) {
	tests := []struct {
		name        string
		counterType TokenCounterType
		text        string
		checkFunc   func(float64) bool
	}{
		{
			name:        "OpenAI normal text",
			counterType: TokenCounterOpenAI,
			text:        "Normal text without excessive whitespace",
			checkFunc: func(adj float64) bool {
				return adj == 1.0
			},
		},
		{
			name:        "OpenAI excessive whitespace",
			counterType: TokenCounterOpenAI,
			text:        "Text  with  many  spaces  everywhere  here",
			checkFunc: func(adj float64) bool {
				return adj >= 1.0
			},
		},
		{
			name:        "Anthropic natural language",
			counterType: TokenCounterAnthropic,
			text:        "This is a natural language sentence. It has good structure.",
			checkFunc: func(adj float64) bool {
				return adj <= 1.0
			},
		},
		{
			name:        "Ollama conservative estimate",
			counterType: TokenCounterOllama,
			text:        "Any text",
			checkFunc: func(adj float64) bool {
				return adj == 1.1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := NewProviderTokenEstimator(tt.counterType)
			adjustment := estimator.getProviderAdjustment(tt.text)
			if !tt.checkFunc(adjustment) {
				t.Errorf("getProviderAdjustment() = %v, check failed", adjustment)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("containsSQL", func(t *testing.T) {
		sqlTexts := []string{
			"select * from users",
			"create table foo (id int)",
			"insert into bar values (1)",
		}
		for _, text := range sqlTexts {
			if !containsSQL(text) {
				t.Errorf("containsSQL(%q) = false, want true", text)
			}
		}

		nonSQLTexts := []string{
			"hello world",
			"just plain text",
		}
		for _, text := range nonSQLTexts {
			if containsSQL(text) {
				t.Errorf("containsSQL(%q) = true, want false", text)
			}
		}
	})

	t.Run("containsJSON", func(t *testing.T) {
		jsonTexts := []string{
			`{"key": "value"}`,
			`[1, 2, 3]`,
		}
		for _, text := range jsonTexts {
			if !containsJSON(text) {
				t.Errorf("containsJSON(%q) = false, want true", text)
			}
		}

		nonJSONTexts := []string{
			"hello world",
			"{not valid json",
		}
		for _, text := range nonJSONTexts {
			if containsJSON(text) {
				t.Errorf("containsJSON(%q) = true, want false", text)
			}
		}
	})

	t.Run("containsCode", func(t *testing.T) {
		codeTexts := []string{
			"function test() {}",
			"const x = 42;",
			"def foo():",
		}
		for _, text := range codeTexts {
			if !containsCode(text) {
				t.Errorf("containsCode(%q) = false, want true", text)
			}
		}

		nonCodeTexts := []string{
			"hello world",
			"just plain text",
		}
		for _, text := range nonCodeTexts {
			if containsCode(text) {
				t.Errorf("containsCode(%q) = true, want false", text)
			}
		}
	})

	t.Run("isNaturalLanguage", func(t *testing.T) {
		naturalTexts := []string{
			"This is a natural sentence with good structure and proper length.",
			"What are you doing today? I am working on a really interesting project that involves databases.",
		}
		for _, text := range naturalTexts {
			if !isNaturalLanguage(text) {
				t.Errorf("isNaturalLanguage(%q) = false, want true", text)
			}
		}

		nonNaturalTexts := []string{
			"x",
			"a b.", // Too few words per sentence
		}
		for _, text := range nonNaturalTexts {
			if isNaturalLanguage(text) {
				t.Errorf("isNaturalLanguage(%q) = true, want false", text)
			}
		}
	})
}
