//go:build windows

package settings

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"live-translator-go/internal/translator"
)

const FileName = "setting.json"

type Values struct {
	Provider            string `json:"provider"`
	APIKey              string `json:"apiKey"`
	BaseURL             string `json:"baseUrl"`
	Model               string `json:"model"`
	TranslationContext  string `json:"translationContext"`
	Glossary            string `json:"glossary"`
	SourceLanguage      string `json:"sourceLanguage"`
	TargetLanguage      string `json:"targetLanguage"`
	CaptionProcessName  string `json:"captionProcessName"`
	CaptionWindowClass  string `json:"captionWindowClass"`
	CaptionAutomationID string `json:"captionAutomationId"`
	CaptionPollMs       int    `json:"captionPollMs"`
	RequestTimeoutMs    int    `json:"requestTimeoutMs"`
	RequestFrequencyMs  int    `json:"requestFrequencyMs"`
	FontFamily          string `json:"fontFamily"`
	FontSize            int    `json:"fontSize"`
	OverlayHeight       int    `json:"overlayHeight"`
	OverlayMarginX      int    `json:"overlayMarginX"`
	OverlayBottomOffset int    `json:"overlayBottomOffset"`
	TextColor           string `json:"textColor"`
	AlternateTextColor  string `json:"alternateTextColor"`
	AlternateLineColors bool   `json:"alternateLineColors"`
	AlwaysOnTop         bool   `json:"alwaysOnTop"`
	ClickThrough        bool   `json:"clickThrough"`
	WordByWord          bool   `json:"wordByWord"`
	ShowOriginal        bool   `json:"showOriginal"`
	StreamingEnabled    bool   `json:"streamingEnabled"`
	UILanguage          string `json:"uiLanguage,omitempty"`

	// Persisted main-window placement. Zero values mean "use default layout".
	WindowX      int `json:"windowX,omitempty"`
	WindowY      int `json:"windowY,omitempty"`
	WindowWidth  int `json:"windowWidth,omitempty"`
	WindowHeight int `json:"windowHeight,omitempty"`
}

func DefaultValues() Values {
	return Values{
		Provider:            translator.DefaultProvider,
		BaseURL:             translator.DefaultBaseURL(translator.DefaultProvider),
		Model:               translator.DefaultModel(translator.DefaultProvider),
		TranslationContext:  "",
		SourceLanguage:      "auto",
		TargetLanguage:      "English",
		CaptionProcessName:  "LiveCaptions",
		CaptionWindowClass:  "LiveCaptionsDesktopWindow",
		CaptionAutomationID: "CaptionsTextBlock",
		CaptionPollMs:       200,
		RequestTimeoutMs:    8000,
		RequestFrequencyMs:  300,
		FontFamily:          "Segoe UI",
		FontSize:            22,
		OverlayHeight:       88,
		OverlayMarginX:      120,
		OverlayBottomOffset: 48,
		TextColor:           "#F5F5F5",
		AlternateTextColor:  "#FFD36A",
		AlternateLineColors: false,
		AlwaysOnTop:         true,
		ClickThrough:        false,
	}
}

func Load() (Values, error) {
	path, err := ResolvePath()
	if err != nil {
		return applyEnvOverrides(Sanitize(DefaultValues())), nil
	}

	values := DefaultValues()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return applyEnvOverrides(Sanitize(values)), nil
		}
		return values, err
	}

	if err := json.Unmarshal(data, &values); err != nil {
		backupPath := path + ".bak"
		_ = os.Remove(backupPath)
		_ = os.Rename(path, backupPath)
		return applyEnvOverrides(Sanitize(DefaultValues())), nil
	}

	return applyEnvOverrides(Sanitize(values)), nil
}

func Save(values Values) error {
	path, err := ResolvePath()
	if err != nil {
		return err
	}

	values = Sanitize(values)
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}

	// 0600: settings file may contain provider API keys; restrict to the current user.
	return os.WriteFile(path, data, 0600)
}

func ResolvePath() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, FileName), nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(executablePath), FileName), nil
}

func IsConfigured(values Values) bool {
	return translator.IsConfigured(values.Provider, values.APIKey, values.Model)
}

func Sanitize(values Values) Values {
	defaults := DefaultValues()
	rawProvider := strings.TrimSpace(values.Provider)
	legacyOpenAI := strings.EqualFold(rawProvider, "openai")
	values.Provider = translator.NormalizeProvider(rawProvider)

	if legacyOpenAI {
		values.APIKey = ""
		values.BaseURL = ""
		values.Model = ""
	}

	values.APIKey = strings.TrimSpace(values.APIKey)
	values.BaseURL = defaultString(values.BaseURL, translator.DefaultBaseURL(values.Provider))
	if translator.UsesModel(values.Provider) {
		values.Model = defaultString(values.Model, translator.DefaultModel(values.Provider))
	} else {
		values.Model = strings.TrimSpace(values.Model)
	}
	values.TranslationContext = strings.TrimSpace(values.TranslationContext)
	values.Glossary = strings.TrimSpace(values.Glossary)
	values.UILanguage = strings.ToLower(strings.TrimSpace(values.UILanguage))
	if values.UILanguage != "pl" && values.UILanguage != "en" {
		values.UILanguage = ""
	}
	values.SourceLanguage = defaultString(values.SourceLanguage, defaults.SourceLanguage)
	values.TargetLanguage = defaultString(values.TargetLanguage, defaults.TargetLanguage)
	values.CaptionProcessName = defaultString(values.CaptionProcessName, defaults.CaptionProcessName)
	values.CaptionWindowClass = defaultString(values.CaptionWindowClass, defaults.CaptionWindowClass)
	values.CaptionAutomationID = defaultString(values.CaptionAutomationID, defaults.CaptionAutomationID)
	values.FontFamily = defaultString(values.FontFamily, defaults.FontFamily)

	values.CaptionPollMs = defaultInt(values.CaptionPollMs, defaults.CaptionPollMs)
	values.RequestTimeoutMs = defaultInt(values.RequestTimeoutMs, defaults.RequestTimeoutMs)
	values.RequestFrequencyMs = defaultInt(values.RequestFrequencyMs, defaults.RequestFrequencyMs)
	values.FontSize = defaultInt(values.FontSize, defaults.FontSize)
	values.OverlayHeight = defaultInt(values.OverlayHeight, defaults.OverlayHeight)
	values.OverlayMarginX = defaultNonNegativeInt(values.OverlayMarginX, defaults.OverlayMarginX)
	values.OverlayBottomOffset = defaultNonNegativeInt(values.OverlayBottomOffset, defaults.OverlayBottomOffset)
	values.TextColor = NormalizeHexColor(values.TextColor, defaults.TextColor)
	values.AlternateTextColor = NormalizeHexColor(values.AlternateTextColor, defaults.AlternateTextColor)

	return values
}

func applyEnvOverrides(values Values) Values {
	values.Provider = envString("LIVE_TRANSLATOR_PROVIDER", values.Provider)
	values.APIKey = envString("LIVE_TRANSLATOR_API_KEY", values.APIKey)
	values.BaseURL = envString("LIVE_TRANSLATOR_BASE_URL", values.BaseURL)
	values.Model = envString("LIVE_TRANSLATOR_MODEL", values.Model)
	values.TranslationContext = envString("LIVE_TRANSLATOR_TRANSLATION_CONTEXT", values.TranslationContext)
	values.SourceLanguage = envString("LIVE_TRANSLATOR_SOURCE_LANGUAGE", values.SourceLanguage)
	values.TargetLanguage = envString("LIVE_TRANSLATOR_TARGET_LANGUAGE", values.TargetLanguage)
	values.CaptionProcessName = envString("LIVE_TRANSLATOR_CAPTIONS_PROCESS", values.CaptionProcessName)
	values.CaptionWindowClass = envString("LIVE_TRANSLATOR_CAPTIONS_WINDOW_CLASS", values.CaptionWindowClass)
	values.CaptionAutomationID = envString("LIVE_TRANSLATOR_CAPTIONS_AUTOMATION_ID", values.CaptionAutomationID)
	values.CaptionPollMs = envInt("LIVE_TRANSLATOR_CAPTION_POLL_MS", values.CaptionPollMs)
	values.RequestTimeoutMs = envInt("LIVE_TRANSLATOR_REQUEST_TIMEOUT_MS", values.RequestTimeoutMs)
	values.FontFamily = envString("LIVE_TRANSLATOR_FONT_FAMILY", values.FontFamily)
	values.FontSize = envInt("LIVE_TRANSLATOR_FONT_SIZE", values.FontSize)
	values.OverlayHeight = envInt("LIVE_TRANSLATOR_OVERLAY_HEIGHT", values.OverlayHeight)
	values.OverlayMarginX = envInt("LIVE_TRANSLATOR_OVERLAY_MARGIN_X", values.OverlayMarginX)
	values.OverlayBottomOffset = envInt("LIVE_TRANSLATOR_OVERLAY_BOTTOM_OFFSET", values.OverlayBottomOffset)
	values.TextColor = envString("LIVE_TRANSLATOR_TEXT_COLOR", values.TextColor)
	values.AlternateTextColor = envString("LIVE_TRANSLATOR_ALTERNATE_TEXT_COLOR", values.AlternateTextColor)
	values.AlternateLineColors = envBool("LIVE_TRANSLATOR_ALTERNATE_LINE_COLORS", values.AlternateLineColors)
	values.AlwaysOnTop = envBool("LIVE_TRANSLATOR_ALWAYS_ON_TOP", values.AlwaysOnTop)
	values.ClickThrough = envBool("LIVE_TRANSLATOR_CLICK_THROUGH", values.ClickThrough)
	values.WordByWord = envBool("LIVE_TRANSLATOR_WORD_BY_WORD", values.WordByWord)
	return Sanitize(values)
}

func IsValidHexColor(value string) bool {
	_, ok := normalizeHexColor(value)
	return ok
}

func NormalizeHexColor(value, fallback string) string {
	normalized, ok := normalizeHexColor(value)
	if ok {
		return normalized
	}
	return fallback
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func defaultString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultNonNegativeInt(value, fallback int) int {
	if value < 0 {
		return fallback
	}
	return value
}

func normalizeHexColor(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	trimmed = strings.TrimPrefix(trimmed, "#")
	if len(trimmed) != 6 {
		return "", false
	}

	upper := strings.ToUpper(trimmed)
	if _, err := hex.DecodeString(upper); err != nil {
		return "", false
	}

	return "#" + upper, true
}
