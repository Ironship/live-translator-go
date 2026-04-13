//go:build windows

package app

import (
	"encoding/hex"
	"strings"
	"time"

	"live-translator-go/internal/captions"
	"live-translator-go/internal/overlay"
	"live-translator-go/internal/settings"
	"live-translator-go/internal/translator"
)

type Config struct {
	Captions         captions.Config
	Translator       translator.Config
	Overlay          overlay.Config
	RequestTimeout   time.Duration
	RequestFrequency time.Duration
}

const fastRefreshRequestFrequency = 50 * time.Millisecond

func LoadSettings() (settings.Values, error) {
	return settings.Load()
}

func ConfigFromSettings(values settings.Values) Config {
	values = settings.Sanitize(values)

	requestFrequency := time.Duration(values.RequestFrequencyMs) * time.Millisecond
	if values.WordByWord {
		requestFrequency = fastRefreshRequestFrequency
	}

	return Config{
		Captions: captions.Config{
			ProcessName:     values.CaptionProcessName,
			WindowClassName: values.CaptionWindowClass,
			AutomationID:    values.CaptionAutomationID,
			PollInterval:    time.Duration(values.CaptionPollMs) * time.Millisecond,
		},
		Translator: translator.Config{
			Provider:       values.Provider,
			BaseURL:        values.BaseURL,
			APIKey:         values.APIKey,
			Model:          values.Model,
			Context:        values.TranslationContext,
			SourceLanguage: values.SourceLanguage,
			TargetLanguage: values.TargetLanguage,
		},
		Overlay: overlay.Config{
			InitialText:         "Waiting for Live Captions...",
			FontFamily:          values.FontFamily,
			FontSize:            values.FontSize,
			Height:              values.OverlayHeight,
			HorizontalMargin:    values.OverlayMarginX,
			BottomOffset:        values.OverlayBottomOffset,
			BackgroundColor:     overlay.Color{R: 18, G: 18, B: 18},
			TextColor:           overlayColorFromHex(values.TextColor, overlay.Color{R: 245, G: 245, B: 245}),
			AlternateTextColor:  overlayColorFromHex(values.AlternateTextColor, overlay.Color{R: 255, G: 211, B: 106}),
			AlternateLineColors: values.AlternateLineColors,
			AlwaysOnTop:         values.AlwaysOnTop,
			ClickThrough:        values.ClickThrough,
		},
		RequestTimeout:   time.Duration(values.RequestTimeoutMs) * time.Millisecond,
		RequestFrequency: requestFrequency,
	}
}

func overlayColorFromHex(value string, fallback overlay.Color) overlay.Color {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(trimmed) != 6 {
		return fallback
	}

	decoded, err := hex.DecodeString(trimmed)
	if err != nil || len(decoded) != 3 {
		return fallback
	}

	return overlay.Color{R: decoded[0], G: decoded[1], B: decoded[2]}
}
