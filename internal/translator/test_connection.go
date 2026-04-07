//go:build windows

package translator

import (
	"context"
	"fmt"
	"strings"
)

func TestConnection(ctx context.Context, config Config) (string, error) {
	config.Provider = NormalizeProvider(config.Provider)
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = DefaultBaseURL(config.Provider)
	}
	if strings.TrimSpace(config.Model) == "" && UsesModel(config.Provider) {
		config.Model = DefaultModel(config.Provider)
	}
	if strings.TrimSpace(config.SourceLanguage) == "" {
		config.SourceLanguage = "auto"
	}
	if strings.TrimSpace(config.TargetLanguage) == "" {
		config.TargetLanguage = "English"
	}

	if !IsConfigured(config.Provider, config.APIKey, config.Model) {
		return "", fmt.Errorf(MissingConfigurationMessage(config.Provider))
	}

	translated, err := config.NewClient().Translate(ctx, "Connection test")
	if err != nil {
		return "", err
	}

	translated = strings.TrimSpace(translated)
	if translated == "" {
		return "", fmt.Errorf("provider returned empty test response")
	}

	return fmt.Sprintf("%s connection OK. Sample response: %s", config.Provider, shortenPreview(translated, 72)), nil
}

func shortenPreview(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}