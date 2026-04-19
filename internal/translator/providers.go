//go:build windows

package translator

import (
	"context"
	"strings"
)

const (
	ProviderOllama   = "Ollama"
	ProviderLMStudio = "LM Studio"
	ProviderGoogle   = "Google"
	ProviderDeepL    = "DeepL"
	DefaultProvider  = ProviderGoogle
)

type Client interface {
	Translate(ctx context.Context, input string) (string, error)
}

func ProviderOptions() []string {
	return []string{
		ProviderOllama,
		ProviderLMStudio,
		ProviderGoogle,
		ProviderDeepL,
	}
}

func NormalizeProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "openai":
		return DefaultProvider
	case "ollama":
		return ProviderOllama
	case "lm studio", "lmstudio":
		return ProviderLMStudio
	case "google":
		return ProviderGoogle
	case "deepl", "deep l":
		return ProviderDeepL
	default:
		return DefaultProvider
	}
}

func DefaultBaseURL(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderOllama:
		return "http://localhost:11434/v1"
	case ProviderLMStudio:
		return "http://127.0.0.1:1234/v1"
	case ProviderDeepL:
		return "https://api-free.deepl.com/v2"
	case ProviderGoogle:
		return "https://translate.googleapis.com"
	default:
		return "https://translate.googleapis.com"
	}
}

func DefaultModel(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderOllama:
		return "llama3.1:8b"
	case ProviderLMStudio:
		return "local-model"
	default:
		return ""
	}
}

func UsesBaseURL(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama, ProviderLMStudio, ProviderDeepL:
		return true
	default:
		return false
	}
}

func UsesModel(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama, ProviderLMStudio:
		return true
	default:
		return false
	}
}

func RequiresAPIKey(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderDeepL:
		return true
	default:
		return false
	}
}

// UsesTranslationContext reports whether the provider can incorporate the
// optional free-form translation context hint (currently only chat-completions
// backends such as Ollama and LM Studio).
func UsesTranslationContext(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama, ProviderLMStudio:
		return true
	default:
		return false
	}
}

// UsesGlossary reports whether the provider honours the user-provided pinned
// term glossary. Currently only chat-completions backends apply it directly in
// the system prompt; DeepL has a separate glossary API that is not yet wired up.
func UsesGlossary(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama, ProviderLMStudio:
		return true
	default:
		return false
	}
}

// SupportsStreaming reports whether the provider can emit partial translations
// as tokens arrive. Only chat-completions backends implement streaming today.
func SupportsStreaming(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama, ProviderLMStudio:
		return true
	default:
		return false
	}
}

func IsConfigured(provider string, apiKey string, model string) bool {
	provider = NormalizeProvider(provider)
	if RequiresAPIKey(provider) && strings.TrimSpace(apiKey) == "" {
		return false
	}
	if UsesModel(provider) && strings.TrimSpace(model) == "" {
		return false
	}
	return true
}

func MissingConfigurationMessage(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDeepL:
		return "Brak klucza API dla DeepL. Kliknij Settings i uzupelnij konfiguracje."
	case ProviderOllama:
		return "Ollama wymaga ustawienia modelu i poprawnego adresu serwera. Kliknij Settings."
	case ProviderLMStudio:
		return "LM Studio wymaga ustawienia modelu i poprawnego adresu serwera. Kliknij Settings."
	case ProviderGoogle:
		return "Google provider jest gotowy po zapisaniu ustawien."
	default:
		return "Brak wymaganej konfiguracji providera. Kliknij Settings."
	}
}

func ProviderHint(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderOllama:
		return "Ollama: pokazuje tylko Server URL i Model. Klucz API nie jest wymagany."
	case ProviderLMStudio:
		return "LM Studio: pokazuje tylko Server URL i Model. Ustaw nazwe aktualnie zaladowanego modelu."
	case ProviderGoogle:
		return "Google: nie wymaga dodatkowych pol providera. Uzywa wbudowanego publicznego endpointu tlumaczen."
	case ProviderDeepL:
		return "DeepL: pokazuje Endpoint URL i API key. Model nie jest uzywany."
	default:
		return "Wybierz provider tlumaczenia i uzupelnij wymagane pola."
	}
}

func APIKeyLabel(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDeepL:
		return "DeepL API key"
	default:
		return "API key / token"
	}
}

func BaseURLLabel(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderDeepL:
		return "Endpoint URL"
	case ProviderOllama, ProviderLMStudio:
		return "Server URL"
	default:
		return "Base URL"
	}
}

func ModelLabel(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderOllama:
		return "Ollama model"
	case ProviderLMStudio:
		return "Loaded model"
	default:
		return "Model"
	}
}

func ProviderIndex(provider string) int {
	normalized := NormalizeProvider(provider)
	for index, option := range ProviderOptions() {
		if option == normalized {
			return index
		}
	}
	return 0
}

func (c Config) NewClient() Client {
	switch NormalizeProvider(c.Provider) {
	case ProviderGoogle:
		return NewGoogleClient(c)
	case ProviderDeepL:
		return NewDeepLClient(c)
	case ProviderOllama, ProviderLMStudio:
		return NewChatCompletionsClient(c)
	default:
		return NewGoogleClient(c)
	}
}
