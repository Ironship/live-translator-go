//go:build windows

package translator

import "strings"

var commonLanguageCodes = map[string]string{
	"auto":       "auto",
	"polish":     "pl",
	"polski":     "pl",
	"pl":         "pl",
	"english":    "en",
	"angielski":  "en",
	"en":         "en",
	"german":     "de",
	"niemiecki":  "de",
	"de":         "de",
	"french":     "fr",
	"francuski":  "fr",
	"fr":         "fr",
	"spanish":    "es",
	"hiszpanski": "es",
	"es":         "es",
	"italian":    "it",
	"wloski":     "it",
	"it":         "it",
	"japanese":   "ja",
	"japonski":   "ja",
	"ja":         "ja",
	"korean":     "ko",
	"koreanski":  "ko",
	"ko":         "ko",
	"ukrainian":  "uk",
	"ukrainski":  "uk",
	"uk":         "uk",
	"russian":    "ru",
	"rosyjski":   "ru",
	"ru":         "ru",
	"chinese":    "zh",
	"chinski":    "zh",
	"zh":         "zh",
	"portuguese": "pt",
	"portugalski": "pt",
	"pt":         "pt",
	"turkish":    "tr",
	"turecki":    "tr",
	"tr":         "tr",
}

func normalizeLanguageCode(value string, fallback string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return fallback
	}
	if code, ok := commonLanguageCodes[trimmed]; ok {
		return code
	}
	return trimmed
}

func googleTargetLanguage(value string) string {
	code := normalizeLanguageCode(value, "en")
	return strings.ToLower(code)
}

func deepLTargetLanguage(value string) string {
	code := normalizeLanguageCode(value, "en")
	switch strings.ToLower(code) {
	case "en-gb":
		return "EN-GB"
	case "en-us":
		return "EN-US"
	case "pt-br":
		return "PT-BR"
	case "pt-pt":
		return "PT-PT"
	case "zh-hans":
		return "ZH-HANS"
	case "zh-hant":
		return "ZH-HANT"
	default:
		return strings.ToUpper(code)
	}
}

func deepLSourceLanguage(value string) string {
	code := normalizeLanguageCode(value, "auto")
	if code == "auto" {
		return ""
	}
	return strings.ToUpper(code)
}