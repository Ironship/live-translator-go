//go:build windows

package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Provider       string
	BaseURL        string
	APIKey         string
	Model          string
	Context        string
	SourceLanguage string
	TargetLanguage string
}

type ChatCompletionsClient struct {
	config     Config
	httpClient *http.Client
}

type chatCompletionRequest struct {
	Model       string                `json:"model"`
	Messages    []chatCompletionEntry `json:"messages"`
	Temperature float64               `json:"temperature"`
}

type chatCompletionEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Message chatCompletionEntry `json:"message"`
}

func NewChatCompletionsClient(config Config) *ChatCompletionsClient {
	config.Provider = NormalizeProvider(config.Provider)
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = DefaultBaseURL(config.Provider)
	}
	if strings.TrimSpace(config.Model) == "" {
		config.Model = DefaultModel(config.Provider)
	}
	if strings.TrimSpace(config.SourceLanguage) == "" {
		config.SourceLanguage = "auto"
	}
	if strings.TrimSpace(config.TargetLanguage) == "" {
		config.TargetLanguage = "English"
	}

	return &ChatCompletionsClient{
		config:     config,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ChatCompletionsClient) Translate(ctx context.Context, input string) (string, error) {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return "", nil
	}

	if RequiresAPIKey(c.config.Provider) && strings.TrimSpace(c.config.APIKey) == "" {
		return "", fmt.Errorf("API key is empty for provider %s", c.config.Provider)
	}
	if UsesModel(c.config.Provider) && strings.TrimSpace(c.config.Model) == "" {
		return "", fmt.Errorf("model is empty for provider %s", c.config.Provider)
	}

	payload := chatCompletionRequest{
		Model: c.config.Model,
		Messages: []chatCompletionEntry{
			{Role: "system", Content: c.systemPrompt()},
			{Role: "user", Content: trimmedInput},
		},
		Temperature: 0.2,
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal translator request: %w", err)
	}

	endpoint := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("create translator request: %w", err)
	}

	if strings.TrimSpace(c.config.APIKey) != "" {
		request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call translator API: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("translator API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var parsed chatCompletionResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode translator response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("translator response contained no choices")
	}

	translated := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if translated == "" {
		return "", fmt.Errorf("translator response contained empty content")
	}

	return translated, nil
}

func (c *ChatCompletionsClient) systemPrompt() string {
	sourceLanguage := strings.TrimSpace(c.config.SourceLanguage)
	if sourceLanguage == "" || strings.EqualFold(sourceLanguage, "auto") {
		sourceLanguage = "the detected language"
	}

	targetLanguage := strings.TrimSpace(c.config.TargetLanguage)
	if targetLanguage == "" {
		targetLanguage = "English"
	}

	translationContext := strings.TrimSpace(c.config.Context)
	if translationContext == "" {
		return fmt.Sprintf(
			"You translate live captions from %s to %s. Return only the translated text. Preserve sentence order and intent. Do not add commentary or quotation marks.",
			sourceLanguage,
			targetLanguage,
		)
	}

	translationContext = strings.ReplaceAll(translationContext, "{source_language}", sourceLanguage)
	translationContext = strings.ReplaceAll(translationContext, "{target_language}", targetLanguage)
	translationContext = strings.ReplaceAll(translationContext, "{context}", "")
	translationContext = strings.ReplaceAll(translationContext, "{target_line}", "")
	translationContext = strings.TrimSpace(translationContext)

	return fmt.Sprintf(
		"You translate live captions from %s to %s. Return only the translated text. Preserve sentence order and intent. Do not add commentary or quotation marks. Use this additional context to resolve ambiguity: %s",
		sourceLanguage,
		targetLanguage,
		translationContext,
	)
}
