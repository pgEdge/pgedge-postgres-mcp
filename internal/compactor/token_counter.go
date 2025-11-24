package compactor

import (
	"strings"
)

// TokenCounterType represents different token counting strategies
type TokenCounterType string

const (
	// TokenCounterGeneric uses generic estimation
	TokenCounterGeneric TokenCounterType = "generic"

	// TokenCounterOpenAI uses OpenAI-specific estimation (GPT-3.5/4 tokenizer)
	TokenCounterOpenAI TokenCounterType = "openai"

	// TokenCounterAnthropic uses Anthropic-specific estimation (Claude tokenizer)
	TokenCounterAnthropic TokenCounterType = "anthropic"

	// TokenCounterOllama uses Ollama-specific estimation
	TokenCounterOllama TokenCounterType = "ollama"
)

// TokenCounterConfig configures the token counter for a specific provider
type TokenCounterConfig struct {
	Type          TokenCounterType
	CharsPerToken float64
	Overhead      int

	// Provider-specific adjustments
	SQLMultiplier  float64
	JSONMultiplier float64
	CodeMultiplier float64
}

// NewTokenCounterConfig creates a config for the specified provider
func NewTokenCounterConfig(counterType TokenCounterType) TokenCounterConfig {
	switch counterType {
	case TokenCounterOpenAI:
		return TokenCounterConfig{
			Type:           TokenCounterOpenAI,
			CharsPerToken:  4.0, // GPT-4 averages ~4 chars/token
			Overhead:       4,   // Message formatting overhead
			SQLMultiplier:  1.15,
			JSONMultiplier: 1.1,
			CodeMultiplier: 1.05,
		}

	case TokenCounterAnthropic:
		return TokenCounterConfig{
			Type:           TokenCounterAnthropic,
			CharsPerToken:  3.8, // Claude tokenizer is slightly more efficient
			Overhead:       5,   // Role/content structure
			SQLMultiplier:  1.2,
			JSONMultiplier: 1.15,
			CodeMultiplier: 1.1,
		}

	case TokenCounterOllama:
		return TokenCounterConfig{
			Type:           TokenCounterOllama,
			CharsPerToken:  4.5, // Varies by model, use conservative estimate
			Overhead:       3,
			SQLMultiplier:  1.1,
			JSONMultiplier: 1.1,
			CodeMultiplier: 1.05,
		}

	default: // TokenCounterGeneric
		return TokenCounterConfig{
			Type:           TokenCounterGeneric,
			CharsPerToken:  4.0,
			Overhead:       10,
			SQLMultiplier:  1.2,
			JSONMultiplier: 1.15,
			CodeMultiplier: 1.1,
		}
	}
}

// ProviderTokenEstimator estimates tokens using provider-specific logic
type ProviderTokenEstimator struct {
	config TokenCounterConfig
}

// NewProviderTokenEstimator creates a token estimator for a specific provider
func NewProviderTokenEstimator(counterType TokenCounterType) *ProviderTokenEstimator {
	return &ProviderTokenEstimator{
		config: NewTokenCounterConfig(counterType),
	}
}

// EstimateTokens estimates tokens using provider-specific logic
func (pte *ProviderTokenEstimator) EstimateTokens(text string) int {
	// Base estimation
	baseTokens := float64(len(text)) / pte.config.CharsPerToken

	// Apply content-type multipliers
	multiplier := pte.getContentMultiplier(text)

	// Provider-specific adjustments
	providerAdjustment := pte.getProviderAdjustment(text)

	total := int(baseTokens * multiplier * providerAdjustment)

	return total + pte.config.Overhead
}

// getContentMultiplier returns multiplier based on content type
func (pte *ProviderTokenEstimator) getContentMultiplier(text string) float64 {
	lowerText := strings.ToLower(text)

	// Check content types in order of specificity
	if containsSQL(lowerText) {
		return pte.config.SQLMultiplier
	}

	if containsJSON(text) {
		return pte.config.JSONMultiplier
	}

	if containsCode(text) {
		return pte.config.CodeMultiplier
	}

	return 1.0
}

// getProviderAdjustment applies provider-specific adjustments
func (pte *ProviderTokenEstimator) getProviderAdjustment(text string) float64 {
	switch pte.config.Type {
	case TokenCounterOpenAI:
		// OpenAI tokenizer handles whitespace efficiently
		// Penalize excessive whitespace
		if strings.Count(text, "  ") > len(text)/100 {
			return 1.05
		}
		return 1.0

	case TokenCounterAnthropic:
		// Claude tokenizer is efficient with natural language
		// Bonus for natural, flowing text
		if isNaturalLanguage(text) {
			return 0.95
		}
		return 1.0

	case TokenCounterOllama:
		// Conservative estimation for Ollama (varies by model)
		return 1.1

	default:
		return 1.0
	}
}

// Helper functions

func containsSQL(text string) bool {
	sqlKeywords := []string{
		"select ", "from ", "where ", "join ",
		"create table", "alter table", "drop table",
		"insert into", "update ", "delete from",
		"group by", "order by", "having ",
	}

	for _, keyword := range sqlKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func containsJSON(text string) bool {
	trimmed := strings.TrimSpace(text)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

func containsCode(text string) bool {
	codePatterns := []string{
		"```", "function ", "const ", "let ", "var ",
		"def ", "class ", "import ", "package ",
	}

	for _, pattern := range codePatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func isNaturalLanguage(text string) bool {
	// Simple heuristic: natural language has reasonable sentence structure
	sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
	words := len(strings.Fields(text))

	if words == 0 {
		return false
	}

	// If we have ~10-20 words per sentence, it's likely natural language
	wordsPerSentence := float64(words) / float64(max(sentences, 1))
	return wordsPerSentence >= 5 && wordsPerSentence <= 30
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
