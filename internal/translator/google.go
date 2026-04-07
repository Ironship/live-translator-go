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

type GoogleClient struct {
	config     Config
	httpClient *http.Client
}

func NewGoogleClient(config Config) *GoogleClient {
	return &GoogleClient{
		config:     config,
		httpClient: &http.Client{},
	}
}

func (c *GoogleClient) Translate(ctx context.Context, input string) (string, error) {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return "", nil
	}

	query := url.Values{}
	query.Set("client", "gtx")
	query.Set("sl", "auto")
	query.Set("tl", googleTargetLanguage(c.config.TargetLanguage))
	query.Set("dt", "t")
	query.Set("q", trimmedInput)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://translate.googleapis.com/translate_a/single?"+query.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("create Google request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call Google Translate: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("Google Translate returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var payload any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode Google Translate response: %w", err)
	}

	translated := extractGoogleTranslation(payload)
	if translated == "" {
		return "", fmt.Errorf("Google Translate response contained no translated text")
	}

	return translated, nil
}

func extractGoogleTranslation(payload any) string {
	root, ok := payload.([]any)
	if !ok || len(root) == 0 {
		return ""
	}

	segments, ok := root[0].([]any)
	if !ok {
		return ""
	}

	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		segmentValues, ok := segment.([]any)
		if !ok || len(segmentValues) == 0 {
			continue
		}
		translated, ok := segmentValues[0].(string)
		if !ok {
			continue
		}
		translated = strings.TrimSpace(translated)
		if translated != "" {
			parts = append(parts, translated)
		}
	}

	return strings.Join(parts, " ")
}