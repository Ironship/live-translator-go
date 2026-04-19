//go:build windows

package translator

import (
	"bufio"
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
	Glossary       string // free-form, one "term|translation" pair per line
}

type ChatCompletionsClient struct {
	config     Config
	httpClient *http.Client
}

type chatCompletionRequest struct {
	Model       string                `json:"model"`
	Messages    []chatCompletionEntry `json:"messages"`
	Temperature float64               `json:"temperature"`
	Stream      bool                  `json:"stream,omitempty"`
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

// TranslateStream issues a streaming chat-completions request and invokes
// onPartial for each incremental delta. The full accumulated translation is
// returned once the stream finishes. onPartial may be nil, in which case the
// method still drains the stream and returns the final text.
func (c *ChatCompletionsClient) TranslateStream(ctx context.Context, input string, onPartial func(partial string)) (string, error) {
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
		Stream:      true,
	}
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal translator stream request: %w", err)
	}

	endpoint := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("create translator stream request: %w", err)
	}
	if strings.TrimSpace(c.config.APIKey) != "" {
		request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call translator stream API: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("translator stream API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	reader := bufio.NewReaderSize(response.Body, 8192)
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" || data == "[DONE]" {
					if data == "[DONE]" {
						break
					}
				} else {
					var frame struct {
						Choices []struct {
							Delta struct {
								Content string `json:"content"`
							} `json:"delta"`
							Message struct {
								Content string `json:"content"`
							} `json:"message"`
						} `json:"choices"`
					}
					if jsonErr := json.Unmarshal([]byte(data), &frame); jsonErr == nil && len(frame.Choices) > 0 {
						delta := frame.Choices[0].Delta.Content
						if delta == "" {
							// Some servers (notably Ollama) omit the delta and
							// include cumulative content inside message.content.
							delta = frame.Choices[0].Message.Content
						}
						if delta != "" {
							builder.WriteString(delta)
							if onPartial != nil {
								onPartial(builder.String())
							}
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("read translator stream: %w", err)
		}
	}

	translated := strings.TrimSpace(builder.String())
	if translated == "" {
		return "", fmt.Errorf("translator stream contained no content")
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

	base := fmt.Sprintf(
		"You translate live captions from %s to %s. Return only the translated text. Preserve sentence order and intent. Do not add commentary or quotation marks.",
		sourceLanguage,
		targetLanguage,
	)

	glossary := formatGlossaryForPrompt(c.config.Glossary)
	if glossary != "" {
		base += " Always honour this glossary of pinned term translations (left = source, right = target): " + glossary + "."
	}

	translationContext := strings.TrimSpace(c.config.Context)
	if translationContext == "" {
		return base
	}

	translationContext = strings.ReplaceAll(translationContext, "{source_language}", sourceLanguage)
	translationContext = strings.ReplaceAll(translationContext, "{target_language}", targetLanguage)
	translationContext = strings.ReplaceAll(translationContext, "{context}", "")
	translationContext = strings.ReplaceAll(translationContext, "{target_line}", "")
	translationContext = strings.TrimSpace(translationContext)

	return base + " Use this additional context to resolve ambiguity: " + translationContext
}

// formatGlossaryForPrompt converts the free-form glossary text (one
// "source|target" or "source=target" or "source\ttarget" pair per line,
// with # and // comments ignored) into a compact single-line representation
// suitable for inclusion in a system prompt.
func formatGlossaryForPrompt(raw string) string {
	entries := ParseGlossary(raw)
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%q -> %q", entry.Source, entry.Target))
	}
	return strings.Join(parts, "; ")
}

// GlossaryEntry represents one pinned translation pair.
type GlossaryEntry struct {
	Source string
	Target string
}

// ParseGlossary parses a free-form glossary string. Each non-empty, non-comment
// line must contain a source and target separated by '|', '=', or a tab.
// Entries with an empty source or target are discarded.
func ParseGlossary(raw string) []GlossaryEntry {
	lines := strings.Split(raw, "\n")
	entries := make([]GlossaryEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		sep := indexOfAny(line, "|=\t")
		if sep <= 0 || sep == len(line)-1 {
			continue
		}
		source := strings.TrimSpace(line[:sep])
		target := strings.TrimSpace(line[sep+1:])
		if source == "" || target == "" {
			continue
		}
		entries = append(entries, GlossaryEntry{Source: source, Target: target})
	}
	return entries
}

func indexOfAny(s, chars string) int {
	for i, r := range s {
		if strings.ContainsRune(chars, r) {
			return i
		}
	}
	return -1
}
