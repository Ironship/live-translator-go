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
// Polish and German strings are intentionally kept ASCII-only (no diacritics
// or umlauts) to match the pre-existing style in this table; some rendering
// paths in the app fall back to fonts that do not ship with the full
// Unicode set, and the existing translations already follow this convention.
var translations = map[string]map[string]string{
	LangEN: {
		"toolbar.start":       "Start / restart Live Captions",
		"toolbar.openPanel":   "Open Windows speech recognition panel",
		"toolbar.settings":    "Settings",
		"toolbar.language":    "Interface language",
		"toolbar.alwaysOnTop": "Always on top",
		"toolbar.wordByWord":  "Word-by-word rendering",
		"toolbar.focus":       "Enter focus mode",
		"toolbar.clear":       "Clear captions",
		"toolbar.exit":        "Exit Live Translator",
		"toolbar.onSuffix":    ": on",
		"toolbar.offSuffix":   ": off",

		"settings.quickSetup": "QUICK SETUP",
		"settings.intro":      "Provider, source window, and preview options are grouped into focused tabs so you can change one thing at a time without hunting through the whole form.",

		"tab.translation": "Translation",
		"tab.captions":    "Live Captions",
		"tab.appearance":  "Appearance",

		"footer.save":         "Save",
		"footer.test":         "Test Connection",
		"footer.close":        "Close",
		"footer.saveTooltip":  "Save settings and apply them immediately",
		"footer.testTooltip":  "Send a small test request to the selected provider",
		"footer.closeTooltip": "Discard unsaved edits and close the settings panel",
		"footer.testing":      "Testing provider connection...",

		"settings.section.provider":      "Translation provider",
		"settings.section.providerNote":  "Choose the backend, then use Test Connection before you close the panel.",
		"settings.field.context":         "Translation context (optional)",
		"settings.field.contextNote":     "Optional: used by Ollama and LM Studio as additional context in the translation prompt.",
		"settings.field.glossary":        "Glossary (optional)",
		"settings.field.glossaryNote":    "One \"source | translation\" pair per line. Lines starting with # are ignored. Used by Ollama / LM Studio to enforce fixed renderings of names and terms.",
		"settings.check.streaming":       "Stream translations incrementally (Ollama / LM Studio only)",
		"settings.section.languages":     "Languages",
		"settings.section.languagesNote": "Leave source on auto unless you have a stable single-language input.",
		"settings.field.sourceLanguage":  "Source language",
		"settings.field.targetLanguage":  "Target language",

		"settings.section.sourceWindow":     "Source window",
		"settings.section.sourceWindowNote": "Defaults match the current Windows 11 Live Captions window. Change these only if Microsoft changes the UI element names.",
		"settings.field.processName":        "Process name",
		"settings.field.windowClass":        "Window class",
		"settings.field.automationId":       "Automation id",
		"settings.section.timing":           "Timing and latency",
		"settings.section.timingNote":       "Lower polling feels snappier but can create more churn when captions change very quickly.",
		"settings.field.captionPollMs":      "Caption poll ms",
		"settings.field.requestTimeoutMs":   "Request timeout ms",
		"settings.field.requestFrequencyMs": "Request frequency ms",
		"settings.check.wordByWord":         "Translate word by word (like Live Captions)",
		"settings.check.wordByWordNote":     "When enabled, translations start immediately on each caption change. Request frequency ms is ignored.",

		"settings.section.preview":     "Preview",
		"settings.section.previewNote": "Font size updates immediately. Use #RRGGBB for line colors, then enable alternating colors if adjacent lines should swap colors.",
		"settings.field.fontSize":      "Font size",
		"settings.field.primaryColor":  "Primary line color",
		"settings.check.alternate":     "Use alternating line colors",
		"settings.check.showOriginal":  "Show original caption alongside translation (bilingual)",
		"settings.field.alternate":     "Alternate line color",
		"settings.check.alwaysOnTop":   "Keep window always on top",
		"settings.check.clickThrough":  "Allow click-through in compact mode",

		"settings.section.language": "Interface language",
		"settings.field.language":   "Language",
		"settings.language":         "Interface language",
		"settings.languageNote":     "Changes take effect the next time the settings panel is opened.",

		"settings.error.pollMs":         "Caption poll ms must be a positive integer.",
		"settings.error.timeoutMs":      "Request timeout ms must be a positive integer.",
		"settings.error.frequencyMs":    "Request frequency ms must be a positive integer.",
		"settings.error.fontSize":       "Font size must be a positive integer.",
		"settings.error.primaryColor":   "Primary line color must use the #RRGGBB format.",
		"settings.error.alternateColor": "Alternate line color must use the #RRGGBB format.",
		"settings.error.save":           "Could not save settings",
	},
	LangPL: {
		"toolbar.start":       "Uruchom / zrestartuj Live Captions",
		"toolbar.openPanel":   "Otworz panel rozpoznawania mowy Windows",
		"toolbar.settings":    "Ustawienia",
		"toolbar.language":    "Jezyk interfejsu",
		"toolbar.alwaysOnTop": "Zawsze na wierzchu",
		"toolbar.wordByWord":  "Renderowanie slowo po slowie",
		"toolbar.focus":       "Wejdz w tryb skupienia",
		"toolbar.clear":       "Wyczysc napisy",
		"toolbar.exit":        "Zamknij Live Translator",
		"toolbar.onSuffix":    ": wl.",
		"toolbar.offSuffix":   ": wyl.",

		"settings.quickSetup": "SZYBKA KONFIGURACJA",
		"settings.intro":      "Opcje dostawcy, zrodla napisow i podgladu sa zgrupowane na osobnych zakladkach, abys mogl zmieniac jedna rzecz naraz bez przeszukiwania calego formularza.",

		"tab.translation": "Tlumaczenie",
		"tab.captions":    "Live Captions",
		"tab.appearance":  "Wyglad",

		"footer.save":         "Zapisz",
		"footer.test":         "Test polaczenia",
		"footer.close":        "Zamknij",
		"footer.saveTooltip":  "Zapisz ustawienia i zastosuj je natychmiast",
		"footer.testTooltip":  "Wyslij niewielkie zapytanie testowe do wybranego dostawcy",
		"footer.closeTooltip": "Odrzuc niezapisane zmiany i zamknij panel ustawien",
		"footer.testing":      "Testowanie polaczenia z dostawca...",

		"settings.section.provider":      "Dostawca tlumaczen",
		"settings.section.providerNote":  "Wybierz backend, a przed zamknieciem panelu uzyj opcji Test polaczenia.",
		"settings.field.context":         "Kontekst tlumaczenia (opcjonalnie)",
		"settings.field.contextNote":     "Opcjonalne: Ollama i LM Studio uzywaja tego jako dodatkowego kontekstu w zapytaniu tlumaczacym.",
		"settings.field.glossary":        "Slownik (opcjonalnie)",
		"settings.field.glossaryNote":    "Jedna para \"zrodlo | tlumaczenie\" na linie. Linie zaczynajace sie od # sa ignorowane. Ollama / LM Studio wykorzystuje ten slownik do wymuszenia ustalonych tlumaczen nazw i terminow.",
		"settings.check.streaming":       "Strumieniuj tlumaczenia przyrostowo (tylko Ollama / LM Studio)",
		"settings.section.languages":     "Jezyki",
		"settings.section.languagesNote": "Pozostaw jezyk zrodlowy na auto, chyba ze masz stabilne zrodlo w jednym jezyku.",
		"settings.field.sourceLanguage":  "Jezyk zrodlowy",
		"settings.field.targetLanguage":  "Jezyk docelowy",

		"settings.section.sourceWindow":     "Okno zrodlowe",
		"settings.section.sourceWindowNote": "Wartosci domyslne pasuja do aktualnego okna Live Captions w Windows 11. Zmieniaj je tylko wtedy, gdy Microsoft zmieni nazwy elementow UI.",
		"settings.field.processName":        "Nazwa procesu",
		"settings.field.windowClass":        "Klasa okna",
		"settings.field.automationId":       "Automation id",
		"settings.section.timing":           "Czas i opoznienia",
		"settings.section.timingNote":       "Rzadsze odpytywanie odczuwalnie przyspiesza reakcje, ale moze powodowac wiecej skokow gdy napisy zmieniaja sie bardzo szybko.",
		"settings.field.captionPollMs":      "Odpytywanie napisow (ms)",
		"settings.field.requestTimeoutMs":   "Limit czasu zapytania (ms)",
		"settings.field.requestFrequencyMs": "Czestotliwosc zapytan (ms)",
		"settings.check.wordByWord":         "Tlumacz slowo po slowie (jak Live Captions)",
		"settings.check.wordByWordNote":     "Gdy wlaczone, tlumaczenia startuja natychmiast przy kazdej zmianie napisow. Czestotliwosc zapytan jest ignorowana.",

		"settings.section.preview":     "Podglad",
		"settings.section.previewNote": "Rozmiar czcionki zmienia sie od razu. Dla kolorow linii uzyj formatu #RRGGBB, a nastepnie wlacz przeplatanie kolorow, jesli sasiednie linie maja miec rozne kolory.",
		"settings.field.fontSize":      "Rozmiar czcionki",
		"settings.field.primaryColor":  "Glowny kolor linii",
		"settings.check.alternate":     "Uzywaj przeplatanych kolorow linii",
		"settings.check.showOriginal":  "Pokazuj oryginalne napisy obok tlumaczenia (dwujezycznie)",
		"settings.field.alternate":     "Alternatywny kolor linii",
		"settings.check.alwaysOnTop":   "Trzymaj okno zawsze na wierzchu",
		"settings.check.clickThrough":  "Pozwol klikac przez okno w trybie kompaktowym",

		"settings.section.language": "Jezyk interfejsu",
		"settings.field.language":   "Jezyk",
		"settings.language":         "Jezyk interfejsu",
		"settings.languageNote":     "Zmiana zostanie zastosowana przy nastepnym otwarciu ustawien.",

		"settings.error.pollMs":         "Odpytywanie napisow (ms) musi byc dodatnia liczba calkowita.",
		"settings.error.timeoutMs":      "Limit czasu zapytania (ms) musi byc dodatnia liczba calkowita.",
		"settings.error.frequencyMs":    "Czestotliwosc zapytan (ms) musi byc dodatnia liczba calkowita.",
		"settings.error.fontSize":       "Rozmiar czcionki musi byc dodatnia liczba calkowita.",
		"settings.error.primaryColor":   "Glowny kolor linii musi miec format #RRGGBB.",
		"settings.error.alternateColor": "Alternatywny kolor linii musi miec format #RRGGBB.",
		"settings.error.save":           "Nie udalo sie zapisac ustawien",
	},
	LangDE: {
		"toolbar.start":       "Live Captions starten / neu starten",
		"toolbar.openPanel":   "Windows-Spracherkennung oeffnen",
		"toolbar.settings":    "Einstellungen",
		"toolbar.language":    "Oberflaechensprache",
		"toolbar.alwaysOnTop": "Immer im Vordergrund",
		"toolbar.wordByWord":  "Wort-fuer-Wort-Darstellung",
		"toolbar.focus":       "Fokusmodus aktivieren",
		"toolbar.clear":       "Untertitel loeschen",
		"toolbar.exit":        "Live Translator beenden",
		"toolbar.onSuffix":    ": ein",
		"toolbar.offSuffix":   ": aus",

		"settings.quickSetup": "SCHNELLSTART",
		"settings.intro":      "Anbieter-, Quellfenster- und Vorschauoptionen sind in fokussierte Registerkarten gruppiert, damit Sie eine Sache nach der anderen aendern koennen, ohne das gesamte Formular zu durchsuchen.",

		"tab.translation": "Uebersetzung",
		"tab.captions":    "Live Captions",
		"tab.appearance":  "Darstellung",

		"footer.save":         "Speichern",
		"footer.test":         "Verbindung testen",
		"footer.close":        "Schliessen",
		"footer.saveTooltip":  "Einstellungen speichern und sofort anwenden",
		"footer.testTooltip":  "Eine kleine Testanfrage an den ausgewaehlten Anbieter senden",
		"footer.closeTooltip": "Nicht gespeicherte Aenderungen verwerfen und das Panel schliessen",
		"footer.testing":      "Verbindung zum Anbieter wird getestet...",

		"settings.section.provider":      "Uebersetzungs-Anbieter",
		"settings.section.providerNote":  "Waehlen Sie das Backend und nutzen Sie Verbindung testen, bevor Sie das Panel schliessen.",
		"settings.field.context":         "Uebersetzungskontext (optional)",
		"settings.field.contextNote":     "Optional: Ollama und LM Studio nutzen diesen Text als zusaetzlichen Kontext im Uebersetzungs-Prompt.",
		"settings.field.glossary":        "Glossar (optional)",
		"settings.field.glossaryNote":    "Ein \"Quelle | Uebersetzung\"-Paar pro Zeile. Zeilen, die mit # beginnen, werden ignoriert. Wird von Ollama / LM Studio verwendet, um feste Uebersetzungen von Namen und Begriffen zu erzwingen.",
		"settings.check.streaming":       "Uebersetzungen inkrementell streamen (nur Ollama / LM Studio)",
		"settings.section.languages":     "Sprachen",
		"settings.section.languagesNote": "Lassen Sie die Quellsprache auf auto, sofern Sie keine stabile Eingabe in einer Sprache haben.",
		"settings.field.sourceLanguage":  "Quellsprache",
		"settings.field.targetLanguage":  "Zielsprache",

		"settings.section.sourceWindow":     "Quellfenster",
		"settings.section.sourceWindowNote": "Die Standardwerte passen zum aktuellen Live-Captions-Fenster unter Windows 11. Aendern Sie sie nur, wenn Microsoft die Namen der UI-Elemente aendert.",
		"settings.field.processName":        "Prozessname",
		"settings.field.windowClass":        "Fensterklasse",
		"settings.field.automationId":       "Automation id",
		"settings.section.timing":           "Timing und Latenz",
		"settings.section.timingNote":       "Selteneres Abfragen fuehlt sich reaktionsfreudiger an, kann aber bei sehr schnellen Untertitelaenderungen mehr Unruhe erzeugen.",
		"settings.field.captionPollMs":      "Untertitel-Abfrage (ms)",
		"settings.field.requestTimeoutMs":   "Anfrage-Timeout (ms)",
		"settings.field.requestFrequencyMs": "Anfragefrequenz (ms)",
		"settings.check.wordByWord":         "Wort fuer Wort uebersetzen (wie Live Captions)",
		"settings.check.wordByWordNote":     "Wenn aktiviert, starten Uebersetzungen sofort bei jeder Untertitelaenderung. Anfragefrequenz wird ignoriert.",

		"settings.section.preview":     "Vorschau",
		"settings.section.previewNote": "Schriftgroesse wird sofort aktualisiert. Nutzen Sie #RRGGBB fuer die Zeilenfarben und aktivieren Sie wechselnde Farben, falls benachbarte Zeilen die Farbe tauschen sollen.",
		"settings.field.fontSize":      "Schriftgroesse",
		"settings.field.primaryColor":  "Primaere Zeilenfarbe",
		"settings.check.alternate":     "Wechselnde Zeilenfarben verwenden",
		"settings.check.showOriginal":  "Originale Untertitel neben der Uebersetzung anzeigen (zweisprachig)",
		"settings.field.alternate":     "Alternative Zeilenfarbe",
		"settings.check.alwaysOnTop":   "Fenster immer im Vordergrund halten",
		"settings.check.clickThrough":  "Durchklicken im Kompaktmodus erlauben",

		"settings.section.language": "Oberflaechensprache",
		"settings.field.language":   "Sprache",
		"settings.language":         "Oberflaechensprache",
		"settings.languageNote":     "Aenderungen werden beim naechsten Oeffnen der Einstellungen wirksam.",

		"settings.error.pollMs":         "Untertitel-Abfrage (ms) muss eine positive Ganzzahl sein.",
		"settings.error.timeoutMs":      "Anfrage-Timeout (ms) muss eine positive Ganzzahl sein.",
		"settings.error.frequencyMs":    "Anfragefrequenz (ms) muss eine positive Ganzzahl sein.",
		"settings.error.fontSize":       "Schriftgroesse muss eine positive Ganzzahl sein.",
		"settings.error.primaryColor":   "Primaere Zeilenfarbe muss das Format #RRGGBB haben.",
		"settings.error.alternateColor": "Alternative Zeilenfarbe muss das Format #RRGGBB haben.",
		"settings.error.save":           "Einstellungen konnten nicht gespeichert werden",
	},
}
