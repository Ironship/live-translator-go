// Package i18n provides a minimal translation table for the user-facing UI
// strings. Translations are compiled in; there is intentionally no file loader
// because the number of strings is small and the supported locales are fixed.
package i18n

import "strings"

// Language identifiers.
const (
	LangEN = "en"
	LangPL = "pl"
	LangDE = "de"
)

// DefaultLanguage is used when no language is explicitly configured.
const DefaultLanguage = LangEN

// SupportedLanguages lists the locales the UI can render, in the order they
// should appear in language pickers and in the toolbar cycle button.
func SupportedLanguages() []string {
	return []string{LangEN, LangPL, LangDE}
}

// DisplayName returns the user-facing label for a language code.
func DisplayName(lang string) string {
	switch Normalize(lang) {
	case LangPL:
		return "Polski"
	case LangDE:
		return "Deutsch"
	default:
		return "English"
	}
}

// Normalize returns a supported language code or DefaultLanguage when the
// input is empty or unknown.
func Normalize(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case LangPL:
		return LangPL
	case LangDE:
		return LangDE
	case LangEN, "":
		return LangEN
	default:
		return DefaultLanguage
	}
}

// NextLanguage returns the language that should be selected after the given
// one when the user clicks the toolbar cycle button. The order matches
// SupportedLanguages and wraps around at the end.
func NextLanguage(lang string) string {
	langs := SupportedLanguages()
	current := Normalize(lang)
	for i, candidate := range langs {
		if candidate == current {
			return langs[(i+1)%len(langs)]
		}
	}
	return langs[0]
}

// T returns the translation for key in the requested language, falling back
// to English and finally to the key itself so missing entries remain visible
// during development.
func T(lang, key string) string {
	lang = Normalize(lang)
	if table, ok := translations[lang]; ok {
		if value, ok := table[key]; ok && value != "" {
			return value
		}
	}
	if lang != LangEN {
		if value, ok := translations[LangEN][key]; ok && value != "" {
			return value
		}
	}
	return key
}

// translations holds the per-language lookup tables. Keys are stable
// identifiers used from call sites; values are the user-facing text.
var translations = map[string]map[string]string{
	LangEN: {
		"toolbar.start":         "Start / restart Live Captions",
		"toolbar.openPanel":     "Open Windows speech recognition panel",
		"toolbar.settings":      "Settings",
		"toolbar.hideSettings":  "Hide settings",
		"toolbar.language":      "Interface language",
		"toolbar.alwaysOnTop":   "Always on top",
		"toolbar.wordByWord":    "Word-by-word rendering",
		"toolbar.focus":         "Enter focus mode",
		"toolbar.exitFocus":     "Exit focus mode (Esc)",
		"toolbar.focusHint":     "Esc exits focus mode",
		"toolbar.clear":         "Clear captions",
		"toolbar.exit":          "Exit Live Translator",
		"toolbar.onSuffix":      ": on",
		"toolbar.offSuffix":     ": off",
		"tab.translation":       "Translation",
		"tab.captions":          "Live Captions",
		"tab.appearance":        "Appearance",
		"footer.save":           "Save",
		"footer.test":           "Test Connection",
		"footer.close":          "Close",
		"settings.language":     "Interface language",
		"settings.languageNote": "Changes take effect the next time the settings panel is opened.",
	},
	LangPL: {
		"toolbar.start":         "Uruchom / zrestartuj Live Captions",
		"toolbar.openPanel":     "Otworz panel rozpoznawania mowy Windows",
		"toolbar.settings":      "Ustawienia",
		"toolbar.hideSettings":  "Ukryj ustawienia",
		"toolbar.language":      "Jezyk interfejsu",
		"toolbar.alwaysOnTop":   "Zawsze na wierzchu",
		"toolbar.wordByWord":    "Renderowanie slowo po slowie",
		"toolbar.focus":         "Wejdz w tryb skupienia",
		"toolbar.exitFocus":     "Wyjdz z trybu skupienia (Esc)",
		"toolbar.focusHint":     "Esc wychodzi z trybu skupienia",
		"toolbar.clear":         "Wyczysc napisy",
		"toolbar.exit":          "Zamknij Live Translator",
		"toolbar.onSuffix":      ": wl.",
		"toolbar.offSuffix":     ": wyl.",
		"tab.translation":       "Tlumaczenie",
		"tab.captions":          "Live Captions",
		"tab.appearance":        "Wyglad",
		"footer.save":           "Zapisz",
		"footer.test":           "Test polaczenia",
		"footer.close":          "Zamknij",
		"settings.language":     "Jezyk interfejsu",
		"settings.languageNote": "Zmiana zostanie zastosowana przy nastepnym otwarciu ustawien.",
	},
	LangDE: {
		"toolbar.start":         "Live Captions starten / neu starten",
		"toolbar.openPanel":     "Windows-Spracherkennung oeffnen",
		"toolbar.settings":      "Einstellungen",
		"toolbar.hideSettings":  "Einstellungen ausblenden",
		"toolbar.language":      "Oberflaechensprache",
		"toolbar.alwaysOnTop":   "Immer im Vordergrund",
		"toolbar.wordByWord":    "Wort-fuer-Wort-Darstellung",
		"toolbar.focus":         "Fokusmodus aktivieren",
		"toolbar.exitFocus":     "Fokusmodus verlassen (Esc)",
		"toolbar.focusHint":     "Esc beendet den Fokusmodus",
		"toolbar.clear":         "Untertitel loeschen",
		"toolbar.exit":          "Live Translator beenden",
		"toolbar.onSuffix":      ": ein",
		"toolbar.offSuffix":     ": aus",
		"tab.translation":       "Uebersetzung",
		"tab.captions":          "Live Captions",
		"tab.appearance":        "Darstellung",
		"footer.save":           "Speichern",
		"footer.test":           "Verbindung testen",
		"footer.close":          "Schliessen",
		"settings.language":     "Oberflaechensprache",
		"settings.languageNote": "Aenderungen werden beim naechsten Oeffnen der Einstellungen wirksam.",
	},
}
