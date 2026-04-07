//go:build windows

package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type DeepLClient struct {
	config     Config
	httpClient *http.Client
}

type deepLResponse struct {
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
}

func NewDeepLClient(config Config) *DeepLClient {
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = DefaultBaseURL(ProviderDeepL)
	}
	return &DeepLClient{
		config:     config,
		httpClient: &http.Client{},
	}
}

func (c *DeepLClient) Translate(ctx context.Context, input string) (string, error) {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return "", nil
	}
	if strings.TrimSpace(c.config.APIKey) == "" {
		return "", fmt.Errorf("DeepL API key is empty")
	}

	form := url.Values{}
	form.Add("text", trimmedInput)
	form.Set("target_lang", deepLTargetLanguage(c.config.TargetLanguage))
	if sourceLang := deepLSourceLanguage(c.config.SourceLanguage); sourceLang != "" {
		form.Set("source_lang", sourceLang)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeDeepLEndpoint(c.config.BaseURL), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create DeepL request: %w", err)
	}
	request.Header.Set("Authorization", "DeepL-Auth-Key "+c.config.APIKey)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call DeepL: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("DeepL returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var parsed deepLResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode DeepL response: %w", err)
	}
	if len(parsed.Translations) == 0 {
		return "", fmt.Errorf("DeepL response contained no translations")
	}

	translated := strings.TrimSpace(parsed.Translations[0].Text)
	if translated == "" {
		return "", fmt.Errorf("DeepL response contained empty translation")
	}

	return translated, nil
}

func normalizeDeepLEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	trimmed = strings.TrimSuffix(trimmed, "/translate")
	if trimmed == "" {
		trimmed = DefaultBaseURL(ProviderDeepL)
	}
	return trimmed + "/translate"
}