//go:build windows

package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"live-translator-go/internal/i18n"
	"live-translator-go/internal/settings"
	"live-translator-go/internal/translator"
	"live-translator-go/internal/ui"

	"github.com/lxn/walk"
)

const labelWidth = 172
const maxFieldHeight = 16777215

type settingsFieldRow struct {
	row   *walk.Composite
	label *walk.Label
	edit  *walk.LineEdit
}

type settingsPanel struct {
	providerButtons   []*walk.PushButton
	providerHint      *walk.Label
	apiKeyRow         *settingsFieldRow
	baseURLRow        *settingsFieldRow
	modelRow          *settingsFieldRow
	contextRow        *settingsFieldRow
	contextNote       *walk.Label
	glossaryRow       *walk.Composite
	glossaryLabel     *walk.Label
	glossaryEdit      *walk.TextEdit
	glossaryNote      *walk.Label
	sourceLangRow     *settingsFieldRow
	targetLangBox     *walk.ComboBox
	pollMsRow         *settingsFieldRow
	timeoutMsRow      *settingsFieldRow
	frequencyMsRow    *settingsFieldRow
	processRow        *settingsFieldRow
	windowClassRow    *settingsFieldRow
	automationIDRow   *settingsFieldRow
	fontSizeRow       *settingsFieldRow
	textColorRow      *settingsFieldRow
	alternateColorRow *settingsFieldRow
	alternateLinesBox *walk.CheckBox
	alwaysOnTopBox    *walk.CheckBox
	clickThroughBox   *walk.CheckBox
	wordByWordBox     *walk.CheckBox
	showOriginalBox   *walk.CheckBox
	streamingBox      *walk.CheckBox
	languageBox       *walk.ComboBox
	statusLabel       *walk.Label
	selectedProvider  string
	base              settings.Values
	onSave            func(settings.Values) error
	onCancel          func()
	providerLabels    []string
	tabFont           *walk.Font
	tabFontSelected   *walk.Font

	// i18n bindings: every translatable widget registers a key and a setter.
	// panel.applyLanguage re-applies them in the currently selected locale.
	lang         string
	i18nBindings []i18nBinding
	tabButtons   []*walk.PushButton
	tabKeys      []string
	currentTab   int
}

// i18nBinding pairs a translation key with a setter that applies the
// resolved text to a specific widget (label, checkbox caption, tooltip, ...).
type i18nBinding struct {
	key   string
	apply func(string)
}

// bind registers a translatable widget and immediately applies the text in
// the panel's current language. The setter is invoked again every time
// applyLanguage is called.
func (p *settingsPanel) bind(key string, apply func(string)) {
	p.i18nBindings = append(p.i18nBindings, i18nBinding{key: key, apply: apply})
	apply(i18n.T(p.lang, key))
}

// bindLabel is a convenience wrapper for widgets whose caption is set via
// walk.Label.SetText (also used for walk.CheckBox captions rendered through
// addSettingsCheckRow's side label).
func (p *settingsPanel) bindLabel(label *walk.Label, key string) {
	if label == nil {
		return
	}
	p.bind(key, func(text string) { _ = label.SetText(text) })
}

// bindButton sets both the visible caption and the tooltip of a push button
// from two different keys. Either key may be empty to skip that part.
func (p *settingsPanel) bindButton(button *walk.PushButton, textKey, tooltipKey, prefix string) {
	if button == nil {
		return
	}
	if textKey != "" {
		p.bind(textKey, func(text string) { _ = button.SetText(prefix + text) })
	}
	if tooltipKey != "" {
		p.bind(tooltipKey, func(text string) { _ = button.SetToolTipText(text) })
	}
}

// applyLanguage refreshes all registered widgets with translations from the
// new locale and redraws the tab bar so its selected/idle glyphs stay
// consistent.
func (p *settingsPanel) applyLanguage(lang string) {
	p.lang = i18n.Normalize(lang)
	for _, b := range p.i18nBindings {
		b.apply(i18n.T(p.lang, b.key))
	}
	p.refreshTabButtons()
}

// refreshTabButtons rerenders the tab bar captions from the current language
// while preserving the selected/idle state.
func (p *settingsPanel) refreshTabButtons() {
	if len(p.tabButtons) == 0 || len(p.tabKeys) != len(p.tabButtons) {
		return
	}
	for i, button := range p.tabButtons {
		if button == nil {
			continue
		}
		title := i18n.T(p.lang, p.tabKeys[i])
		if i == p.currentTab {
			_ = button.SetText(ui.BulletSelected + title)
		} else {
			_ = button.SetText(ui.BulletIdle + title)
		}
	}
}

func newSettingsPanel(parent walk.Container, current settings.Values, onSave func(settings.Values) error, onCancel func()) (*settingsPanel, error) {
	panel := &settingsPanel{onSave: onSave, onCancel: onCancel, lang: i18n.Normalize(current.UILanguage)}

	var panelBrush *walk.SolidColorBrush
	var sectionBrush *walk.SolidColorBrush
	var inputBrush *walk.SolidColorBrush
	if owner, ok := parent.(interface{ AddDisposable(walk.Disposable) }); ok {
		brush, err := walk.NewSolidColorBrush(ui.PanelBackground)
		if err != nil {
			return nil, err
		}
		owner.AddDisposable(brush)
		panelBrush = brush

		brush, err = walk.NewSolidColorBrush(ui.CardBackground)
		if err != nil {
			return nil, err
		}
		owner.AddDisposable(brush)
		sectionBrush = brush

		brush, err = walk.NewSolidColorBrush(ui.InputBackground)
		if err != nil {
			return nil, err
		}
		owner.AddDisposable(brush)
		inputBrush = brush
	}

	// Fonts: dedicated styles for eyebrow, heading, body, and actions so the
	// panel looks typographically organised rather than using walk defaults.
	fontOwner, _ := parent.(interface{ AddDisposable(walk.Disposable) })
	mkFont := func(family string, size int, style walk.FontStyle) *walk.Font {
		f, err := walk.NewFont(family, size, style)
		if err != nil {
			return nil
		}
		if fontOwner != nil {
			fontOwner.AddDisposable(f)
		}
		return f
	}
	eyebrowFont := mkFont("Bahnschrift SemiCondensed", 10, walk.FontBold)
	introFont := mkFont("Segoe UI", 10, 0)
	headingFont := mkFont("Segoe UI Semibold", 12, walk.FontBold)
	bodyFont := mkFont("Segoe UI", 10, 0)
	tabFont := mkFont("Segoe UI", 10, 0)
	tabFontSelected := mkFont("Segoe UI Semibold", 10, walk.FontBold)
	footerFont := mkFont("Segoe UI Semibold", 10, walk.FontBold)

	layout := walk.NewVBoxLayout()
	if err := layout.SetSpacing(16); err != nil {
		return nil, err
	}
	if err := layout.SetMargins(walk.Margins{HNear: 0, VNear: 4, HFar: 0, VFar: 0}); err != nil {
		return nil, err
	}
	if err := parent.SetLayout(layout); err != nil {
		return nil, err
	}
	if panelBrush != nil {
		parent.SetBackground(panelBrush)
	}

	sectionEyebrow, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
	}
	if eyebrowFont != nil {
		sectionEyebrow.SetFont(eyebrowFont)
	}
	sectionEyebrow.SetTextColor(ui.AccentSoft)
	panel.bindLabel(sectionEyebrow, "settings.quickSetup")

	intro, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
	}
	if introFont != nil {
		intro.SetFont(introFont)
	}
	intro.SetTextColor(ui.TextSecondary)
	panel.bindLabel(intro, "settings.intro")

	tabsHost, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	if panelBrush != nil {
		tabsHost.SetBackground(panelBrush)
	}
	tabsLayout := walk.NewVBoxLayout()
	if err := tabsLayout.SetSpacing(12); err != nil {
		return nil, err
	}
	if err := tabsLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := tabsHost.SetLayout(tabsLayout); err != nil {
		return nil, err
	}

	tabRow, err := walk.NewComposite(tabsHost)
	if err != nil {
		return nil, err
	}
	if panelBrush != nil {
		tabRow.SetBackground(panelBrush)
	}
	tabRowLayout := walk.NewHBoxLayout()
	if err := tabRowLayout.SetSpacing(8); err != nil {
		return nil, err
	}
	if err := tabRowLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := tabRow.SetLayout(tabRowLayout); err != nil {
		return nil, err
	}

	translationTabButton, err := walk.NewPushButton(tabRow)
	if err != nil {
		return nil, err
	}
	if tabFont != nil {
		translationTabButton.SetFont(tabFont)
	}
	if err := translationTabButton.SetMinMaxSize(walk.Size{Width: 156, Height: 38}, walk.Size{Width: 16777215, Height: 38}); err != nil {
		return nil, err
	}

	captionsTabButton, err := walk.NewPushButton(tabRow)
	if err != nil {
		return nil, err
	}
	if tabFont != nil {
		captionsTabButton.SetFont(tabFont)
	}
	if err := captionsTabButton.SetMinMaxSize(walk.Size{Width: 156, Height: 38}, walk.Size{Width: 16777215, Height: 38}); err != nil {
		return nil, err
	}

	appearanceTabButton, err := walk.NewPushButton(tabRow)
	if err != nil {
		return nil, err
	}
	if tabFont != nil {
		appearanceTabButton.SetFont(tabFont)
	}
	if err := appearanceTabButton.SetMinMaxSize(walk.Size{Width: 156, Height: 38}, walk.Size{Width: 16777215, Height: 38}); err != nil {
		return nil, err
	}

	pagesHost, err := walk.NewComposite(tabsHost)
	if err != nil {
		return nil, err
	}
	if panelBrush != nil {
		pagesHost.SetBackground(panelBrush)
	}
	if err := tabsLayout.SetStretchFactor(pagesHost, 1); err != nil {
		return nil, err
	}
	pagesLayout := walk.NewVBoxLayout()
	if err := pagesLayout.SetSpacing(0); err != nil {
		return nil, err
	}
	if err := pagesLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := pagesHost.SetLayout(pagesLayout); err != nil {
		return nil, err
	}

	translationPage, err := newSettingsPage(pagesHost, panelBrush)
	if err != nil {
		return nil, err
	}
	translationGroup, translationHeading, err := newSettingsSection(translationPage, "", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(translationHeading, "settings.section.provider")
	translationNote, err := addSettingsGroupNote(translationGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(translationNote, "settings.section.providerNote")
	panel.providerButtons, err = addSettingsProviderRow(translationGroup, translator.ProviderOptions(), current.Provider, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.providerHint, err = walk.NewLabel(translationGroup)
	if err != nil {
		return nil, err
	}
	if bodyFont != nil {
		panel.providerHint.SetFont(bodyFont)
	}
	panel.providerHint.SetTextColor(ui.TextSecondary)
	panel.apiKeyRow, err = addSettingsLineEditRow(translationGroup, translator.APIKeyLabel(current.Provider), current.APIKey, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.apiKeyRow.edit.SetPasswordMode(true)
	panel.baseURLRow, err = addSettingsLineEditRow(translationGroup, translator.BaseURLLabel(current.Provider), current.BaseURL, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.modelRow, err = addSettingsLineEditRow(translationGroup, translator.ModelLabel(current.Provider), current.Model, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.contextRow, err = addSettingsLineEditRow(translationGroup, "", current.TranslationContext, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.contextRow.label, "settings.field.context")
	panel.contextNote, err = addSettingsGroupNote(translationGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.contextNote, "settings.field.contextNote")

	// Glossary (pinned term translations). Only applies to chat-completions backends.
	panel.glossaryRow, panel.glossaryLabel, panel.glossaryEdit, err = addSettingsTextAreaRow(
		translationGroup,
		"",
		current.Glossary,
		inputBrush,
		sectionBrush,
	)
	if err != nil {
		return nil, err
	}
	if bodyFont != nil {
		panel.glossaryLabel.SetFont(bodyFont)
	}
	panel.bindLabel(panel.glossaryLabel, "settings.field.glossary")
	panel.glossaryNote, err = addSettingsGroupNote(
		translationGroup,
		"",
		bodyFont,
	)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.glossaryNote, "settings.field.glossaryNote")

	var streamingLabel *walk.Label
	panel.streamingBox, streamingLabel, err = addSettingsCheckRow(translationGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(streamingLabel, "settings.check.streaming")

	languagesGroup, languagesHeading, err := newSettingsSection(translationPage, "", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(languagesHeading, "settings.section.languages")
	languagesNote, err := addSettingsGroupNote(languagesGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(languagesNote, "settings.section.languagesNote")
	panel.sourceLangRow, err = addSettingsLineEditRow(languagesGroup, "", current.SourceLanguage, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.sourceLangRow.label, "settings.field.sourceLanguage")
	targetLanguageOptions := buildTargetLanguageOptions(current.TargetLanguage)
	var targetLangLabel *walk.Label
	panel.targetLangBox, targetLangLabel, err = addSettingsComboBoxRow(languagesGroup, "", targetLanguageOptions, translator.CanonicalTargetLanguage(current.TargetLanguage), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(targetLangLabel, "settings.field.targetLanguage")

	captionsPage, err := newSettingsPage(pagesHost, panelBrush)
	if err != nil {
		return nil, err
	}
	windowGroup, windowHeading, err := newSettingsSection(captionsPage, "", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(windowHeading, "settings.section.sourceWindow")
	windowNote, err := addSettingsGroupNote(windowGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(windowNote, "settings.section.sourceWindowNote")
	panel.processRow, err = addSettingsLineEditRow(windowGroup, "", current.CaptionProcessName, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.processRow.label, "settings.field.processName")
	panel.windowClassRow, err = addSettingsLineEditRow(windowGroup, "", current.CaptionWindowClass, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.windowClassRow.label, "settings.field.windowClass")
	panel.automationIDRow, err = addSettingsLineEditRow(windowGroup, "", current.CaptionAutomationID, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.automationIDRow.label, "settings.field.automationId")

	timingGroup, timingHeading, err := newSettingsSection(captionsPage, "", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(timingHeading, "settings.section.timing")
	timingNote, err := addSettingsGroupNote(timingGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(timingNote, "settings.section.timingNote")
	panel.pollMsRow, err = addSettingsLineEditRow(timingGroup, "", strconv.Itoa(current.CaptionPollMs), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.pollMsRow.label, "settings.field.captionPollMs")
	panel.timeoutMsRow, err = addSettingsLineEditRow(timingGroup, "", strconv.Itoa(current.RequestTimeoutMs), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.timeoutMsRow.label, "settings.field.requestTimeoutMs")
	panel.frequencyMsRow, err = addSettingsLineEditRow(timingGroup, "", strconv.Itoa(current.RequestFrequencyMs), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.frequencyMsRow.label, "settings.field.requestFrequencyMs")
	var wordByWordLabel *walk.Label
	panel.wordByWordBox, wordByWordLabel, err = addSettingsCheckRow(timingGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(wordByWordLabel, "settings.check.wordByWord")
	wordByWordNote, err := addSettingsGroupNote(timingGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(wordByWordNote, "settings.check.wordByWordNote")

	appearancePage, err := newSettingsPage(pagesHost, panelBrush)
	if err != nil {
		return nil, err
	}
	previewGroup, previewHeading, err := newSettingsSection(appearancePage, "", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(previewHeading, "settings.section.preview")
	previewNote, err := addSettingsGroupNote(previewGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(previewNote, "settings.section.previewNote")
	panel.fontSizeRow, err = addSettingsLineEditRow(previewGroup, "", strconv.Itoa(current.FontSize), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.fontSizeRow.label, "settings.field.fontSize")
	panel.textColorRow, err = addSettingsLineEditRow(previewGroup, "", current.TextColor, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.textColorRow.label, "settings.field.primaryColor")
	var alternateBoxLabel *walk.Label
	panel.alternateLinesBox, alternateBoxLabel, err = addSettingsCheckRow(previewGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(alternateBoxLabel, "settings.check.alternate")
	var showOriginalLabel *walk.Label
	panel.showOriginalBox, showOriginalLabel, err = addSettingsCheckRow(previewGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(showOriginalLabel, "settings.check.showOriginal")
	panel.alternateColorRow, err = addSettingsLineEditRow(previewGroup, "", current.AlternateTextColor, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(panel.alternateColorRow.label, "settings.field.alternate")
	var alwaysOnTopLabel *walk.Label
	panel.alwaysOnTopBox, alwaysOnTopLabel, err = addSettingsCheckRow(previewGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(alwaysOnTopLabel, "settings.check.alwaysOnTop")
	var clickThroughLabel *walk.Label
	panel.clickThroughBox, clickThroughLabel, err = addSettingsCheckRow(previewGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(clickThroughLabel, "settings.check.clickThrough")
	panel.alternateLinesBox.CheckedChanged().Attach(func() {
		panel.updateAppearanceRows()
	})

	languageGroup, languageHeading, err := newSettingsSection(appearancePage, "", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(languageHeading, "settings.section.language")
	languageCodes := i18n.SupportedLanguages()
	languageOptions := make([]string, len(languageCodes))
	for i, code := range languageCodes {
		languageOptions[i] = i18n.DisplayName(code)
	}
	currentLanguageIdx := 0
	currentLangCode := i18n.Normalize(current.UILanguage)
	for i, code := range languageCodes {
		if code == currentLangCode {
			currentLanguageIdx = i
			break
		}
	}
	var languageLabel *walk.Label
	panel.languageBox, languageLabel, err = addSettingsComboBoxRow(languageGroup, "", languageOptions, languageOptions[currentLanguageIdx], inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(languageLabel, "settings.field.language")
	languageNote, err := addSettingsGroupNote(languageGroup, "", bodyFont)
	if err != nil {
		return nil, err
	}
	panel.bindLabel(languageNote, "settings.languageNote")

	footer, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	footerLayout := walk.NewHBoxLayout()
	if err := footerLayout.SetSpacing(12); err != nil {
		return nil, err
	}
	if err := footerLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := footer.SetLayout(footerLayout); err != nil {
		return nil, err
	}
	if panelBrush != nil {
		footer.SetBackground(panelBrush)
	}

	panel.statusLabel, err = walk.NewLabel(footer)
	if err != nil {
		return nil, err
	}
	if bodyFont != nil {
		panel.statusLabel.SetFont(bodyFont)
	}
	panel.statusLabel.SetTextColor(ui.Error)
	if err := footerLayout.SetStretchFactor(panel.statusLabel, 1); err != nil {
		return nil, err
	}

	applyButton, err := walk.NewPushButton(footer)
	if err != nil {
		return nil, err
	}
	if footerFont != nil {
		applyButton.SetFont(footerFont)
	}
	panel.bindButton(applyButton, "footer.save", "footer.saveTooltip", ui.SymbolSave)
	if err := applyButton.SetMinMaxSize(walk.Size{Width: 124, Height: 40}, walk.Size{Width: 164, Height: 40}); err != nil {
		return nil, err
	}

	testButton, err := walk.NewPushButton(footer)
	if err != nil {
		return nil, err
	}
	if footerFont != nil {
		testButton.SetFont(footerFont)
	}
	panel.bindButton(testButton, "footer.test", "footer.testTooltip", ui.SymbolTest)
	if err := testButton.SetMinMaxSize(walk.Size{Width: 176, Height: 40}, walk.Size{Width: 216, Height: 40}); err != nil {
		return nil, err
	}

	cancelButton, err := walk.NewPushButton(footer)
	if err != nil {
		return nil, err
	}
	if footerFont != nil {
		cancelButton.SetFont(footerFont)
	}
	panel.bindButton(cancelButton, "footer.close", "footer.closeTooltip", ui.SymbolCancel)
	if err := cancelButton.SetMinMaxSize(walk.Size{Width: 124, Height: 40}, walk.Size{Width: 164, Height: 40}); err != nil {
		return nil, err
	}

	ui.ApplyNativeDarkTheme(
		translationTabButton,
		captionsTabButton,
		appearanceTabButton,
		panel.alternateLinesBox,
		panel.alwaysOnTopBox,
		panel.clickThroughBox,
		panel.wordByWordBox,
		panel.showOriginalBox,
		panel.streamingBox,
		applyButton,
		testButton,
		cancelButton,
	)
	for _, button := range panel.providerButtons {
		ui.ApplyNativeDarkTheme(button)
	}

	panel.tabButtons = []*walk.PushButton{translationTabButton, captionsTabButton, appearanceTabButton}
	panel.tabKeys = []string{"tab.translation", "tab.captions", "tab.appearance"}
	tabPages := []*walk.Composite{translationPage, captionsPage, appearancePage}
	panel.tabFont = tabFont
	panel.tabFontSelected = tabFontSelected
	showTab := func(index int) {
		panel.currentTab = index
		for i, page := range tabPages {
			page.SetVisible(i == index)
			if i == index {
				if tabFontSelected != nil {
					panel.tabButtons[i].SetFont(tabFontSelected)
				}
			} else {
				if tabFont != nil {
					panel.tabButtons[i].SetFont(tabFont)
				}
			}
		}
		panel.refreshTabButtons()
	}

	translationTabButton.Clicked().Attach(func() { showTab(0) })
	captionsTabButton.Clicked().Attach(func() { showTab(1) })
	appearanceTabButton.Clicked().Attach(func() { showTab(2) })

	// Snapshot original provider labels so we can redraw selection state without
	// accumulating bullet prefixes on repeated toggles.
	panel.providerLabels = make([]string, len(panel.providerButtons))
	for i, button := range panel.providerButtons {
		panel.providerLabels[i] = button.Text()
	}
	for i, button := range panel.providerButtons {
		providerName := panel.providerLabels[i]
		button.Clicked().Attach(func() {
			nextProvider := translator.NormalizeProvider(providerName)
			applySettingsProviderDefaults(panel.selectedProvider, nextProvider, panel.baseURLRow.edit, panel.modelRow.edit)
			panel.selectedProvider = nextProvider
			panel.updateProviderButtons(nextProvider)
			panel.updateProviderRows(nextProvider)
		})
	}

	collectAndValidate := func() (settings.Values, bool) {
		updated, validationMessage := collectPanelSettings(
			panel.base,
			panel.selectedProvider,
			panel.apiKeyRow.edit.Text(),
			panel.baseURLRow.edit.Text(),
			panel.modelRow.edit.Text(),
			panel.contextRow.edit.Text(),
			panel.glossaryText(),
			panel.sourceLangRow.edit.Text(),
			panel.selectedTargetLanguage(),
			panel.pollMsRow.edit.Text(),
			panel.timeoutMsRow.edit.Text(),
			panel.frequencyMsRow.edit.Text(),
			panel.processRow.edit.Text(),
			panel.windowClassRow.edit.Text(),
			panel.automationIDRow.edit.Text(),
			panel.fontSizeRow.edit.Text(),
			panel.textColorRow.edit.Text(),
			panel.alternateColorRow.edit.Text(),
			panel.alternateLinesBox.Checked(),
			panel.alwaysOnTopBox.Checked(),
			panel.clickThroughBox.Checked(),
			panel.wordByWordBox.Checked(),
			panel.showOriginalBox.Checked(),
			panel.selectedLanguageCode(),
			panel.streamingBox.Checked(),
		)
		if validationMessage != "" {
			panel.showError(validationMessage)
			return settings.Values{}, false
		}
		if !settings.IsConfigured(updated) {
			panel.showError(translator.MissingConfigurationMessage(updated.Provider))
			return settings.Values{}, false
		}
		return updated, true
	}

	applyButton.Clicked().Attach(func() {
		updated, ok := collectAndValidate()
		if !ok {
			return
		}
		if panel.onSave != nil {
			if err := panel.onSave(updated); err != nil {
				panel.showError(err.Error())
				return
			}
		}
		panel.base = updated
		panel.clearStatus()
	})

	testButton.Clicked().Attach(func() {
		updated, ok := collectAndValidate()
		if !ok {
			return
		}

		panel.showInfo(i18n.T(panel.lang, "footer.testing"))
		testButton.SetEnabled(false)

		go func(values settings.Values) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(values.RequestTimeoutMs)*time.Millisecond)
			defer cancel()

			message, err := translator.TestConnection(ctx, translator.Config{
				Provider:       values.Provider,
				BaseURL:        values.BaseURL,
				APIKey:         values.APIKey,
				Model:          values.Model,
				Context:        values.TranslationContext,
				SourceLanguage: values.SourceLanguage,
				TargetLanguage: values.TargetLanguage,
				Glossary:       values.Glossary,
			})

			panel.statusLabel.Synchronize(func() {
				if panel.statusLabel.IsDisposed() || testButton.IsDisposed() {
					return
				}

				testButton.SetEnabled(true)
				if err != nil {
					panel.showError(err.Error())
					return
				}

				panel.showSuccess(message)
			})
		}(updated)
	})

	cancelButton.Clicked().Attach(func() {
		panel.Load(panel.base)
		panel.clearStatus()
		if panel.onCancel != nil {
			panel.onCancel()
		}
	})

	showTab(0)
	panel.Load(current)
	return panel, nil
}

func (p *settingsPanel) Load(values settings.Values) {
	values = settings.Sanitize(values)
	p.base = values
	p.selectedProvider = values.Provider
	p.updateProviderButtons(values.Provider)
	_ = p.apiKeyRow.edit.SetText(values.APIKey)
	_ = p.baseURLRow.edit.SetText(values.BaseURL)
	_ = p.modelRow.edit.SetText(values.Model)
	_ = p.contextRow.edit.SetText(values.TranslationContext)
	if p.glossaryEdit != nil {
		_ = p.glossaryEdit.SetText(values.Glossary)
	}
	_ = p.sourceLangRow.edit.SetText(values.SourceLanguage)
	targetLanguageOptions := buildTargetLanguageOptions(values.TargetLanguage)
	_ = p.targetLangBox.SetModel(targetLanguageOptions)
	_ = p.targetLangBox.SetCurrentIndex(indexOfString(targetLanguageOptions, translator.CanonicalTargetLanguage(values.TargetLanguage)))
	_ = p.pollMsRow.edit.SetText(strconv.Itoa(values.CaptionPollMs))
	_ = p.timeoutMsRow.edit.SetText(strconv.Itoa(values.RequestTimeoutMs))
	_ = p.frequencyMsRow.edit.SetText(strconv.Itoa(values.RequestFrequencyMs))
	_ = p.processRow.edit.SetText(values.CaptionProcessName)
	_ = p.windowClassRow.edit.SetText(values.CaptionWindowClass)
	_ = p.automationIDRow.edit.SetText(values.CaptionAutomationID)
	_ = p.fontSizeRow.edit.SetText(strconv.Itoa(values.FontSize))
	_ = p.textColorRow.edit.SetText(values.TextColor)
	_ = p.alternateColorRow.edit.SetText(values.AlternateTextColor)
	p.alternateLinesBox.SetChecked(values.AlternateLineColors)
	p.alwaysOnTopBox.SetChecked(values.AlwaysOnTop)
	p.clickThroughBox.SetChecked(values.ClickThrough)
	p.wordByWordBox.SetChecked(values.WordByWord)
	if p.showOriginalBox != nil {
		p.showOriginalBox.SetChecked(values.ShowOriginal)
	}
	if p.streamingBox != nil {
		p.streamingBox.SetChecked(values.StreamingEnabled)
	}
	if p.languageBox != nil {
		codes := i18n.SupportedLanguages()
		idx := 0
		currentCode := i18n.Normalize(values.UILanguage)
		for i, code := range codes {
			if code == currentCode {
				idx = i
				break
			}
		}
		_ = p.languageBox.SetCurrentIndex(idx)
	}
	p.updateProviderRows(values.Provider)
	p.updateAppearanceRows()
	p.applyLanguage(values.UILanguage)
	p.clearStatus()
}

func (p *settingsPanel) updateProviderRows(provider string) {
	normalized := translator.NormalizeProvider(provider)
	_ = p.providerHint.SetText(translator.ProviderHint(normalized))
	_ = p.apiKeyRow.label.SetText(translator.APIKeyLabel(normalized))
	p.apiKeyRow.row.SetVisible(translator.RequiresAPIKey(normalized))
	_ = p.baseURLRow.label.SetText(translator.BaseURLLabel(normalized))
	p.baseURLRow.row.SetVisible(translator.UsesBaseURL(normalized))
	_ = p.modelRow.label.SetText(translator.ModelLabel(normalized))
	p.modelRow.row.SetVisible(translator.UsesModel(normalized))
	usesContext := translator.UsesTranslationContext(normalized)
	p.contextRow.row.SetVisible(usesContext)
	if p.contextNote != nil {
		p.contextNote.SetVisible(usesContext)
	}
	usesGlossary := translator.UsesGlossary(normalized)
	if p.glossaryRow != nil {
		p.glossaryRow.SetVisible(usesGlossary)
	}
	if p.glossaryNote != nil {
		p.glossaryNote.SetVisible(usesGlossary)
	}
	if p.streamingBox != nil {
		p.streamingBox.SetVisible(translator.SupportsStreaming(normalized))
	}
}

func (p *settingsPanel) selectedTargetLanguage() string {
	if p.targetLangBox == nil {
		return "English"
	}

	return translator.CanonicalTargetLanguage(p.targetLangBox.Text())
}

func (p *settingsPanel) glossaryText() string {
	if p == nil || p.glossaryEdit == nil {
		return ""
	}
	return p.glossaryEdit.Text()
}

func (p *settingsPanel) selectedLanguageCode() string {
	if p == nil || p.languageBox == nil {
		return i18n.DefaultLanguage
	}
	codes := i18n.SupportedLanguages()
	idx := p.languageBox.CurrentIndex()
	if idx >= 0 && idx < len(codes) {
		return codes[idx]
	}
	return i18n.DefaultLanguage
}

func (p *settingsPanel) updateProviderButtons(provider string) {
	normalized := translator.NormalizeProvider(provider)
	for i, button := range p.providerButtons {
		label := p.providerLabels[i]
		if translator.NormalizeProvider(label) == normalized {
			_ = button.SetText(ui.BulletSelected + label)
			if p.tabFontSelected != nil {
				button.SetFont(p.tabFontSelected)
			}
		} else {
			_ = button.SetText(ui.BulletIdle + label)
			if p.tabFont != nil {
				button.SetFont(p.tabFont)
			}
		}
	}
}

func (p *settingsPanel) updateAppearanceRows() {
	if p.alternateColorRow == nil || p.alternateLinesBox == nil {
		return
	}
	p.alternateColorRow.row.SetVisible(p.alternateLinesBox.Checked())
}

func (p *settingsPanel) clearStatus() {
	p.statusLabel.SetTextColor(ui.Error)
	_ = p.statusLabel.SetText("")
}

func (p *settingsPanel) showError(message string) {
	p.statusLabel.SetTextColor(ui.Error)
	_ = p.statusLabel.SetText(message)
}

func (p *settingsPanel) showInfo(message string) {
	p.statusLabel.SetTextColor(ui.Info)
	_ = p.statusLabel.SetText(message)
}

func (p *settingsPanel) showSuccess(message string) {
	p.statusLabel.SetTextColor(ui.Success)
	_ = p.statusLabel.SetText(message)
}

func newSettingsPage(parent walk.Container, background walk.Brush) (*walk.Composite, error) {
	page, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	layout := walk.NewVBoxLayout()
	if err := layout.SetSpacing(14); err != nil {
		return nil, err
	}
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := page.SetLayout(layout); err != nil {
		return nil, err
	}
	if background != nil {
		page.SetBackground(background)
	}
	return page, nil
}

func newSettingsSection(parent walk.Container, title string, background walk.Brush, headingFont, titleFont *walk.Font) (*walk.Composite, *walk.Label, error) {
	group, err := walk.NewComposite(parent)
	if err != nil {
		return nil, nil, err
	}
	if background != nil {
		group.SetBackground(background)
	}
	layout := walk.NewVBoxLayout()
	if err := layout.SetSpacing(12); err != nil {
		return nil, nil, err
	}
	if err := layout.SetMargins(walk.Margins{HNear: 18, VNear: 18, HFar: 18, VFar: 18}); err != nil {
		return nil, nil, err
	}
	if err := group.SetLayout(layout); err != nil {
		return nil, nil, err
	}
	heading, err := walk.NewLabel(group)
	if err != nil {
		return nil, nil, err
	}
	if headingFont != nil {
		heading.SetFont(headingFont)
	} else if titleFont != nil {
		heading.SetFont(titleFont)
	}
	heading.SetTextColor(ui.TextPrimary)
	_ = heading.SetText(title)

	divider, err := walk.NewHSeparator(group)
	if err == nil {
		_ = divider.SetMinMaxSize(walk.Size{Width: 0, Height: 1}, walk.Size{Width: 16777215, Height: 1})
	}
	return group, heading, nil
}

func addSettingsGroupNote(parent walk.Container, text string, font *walk.Font) (*walk.Label, error) {
	note, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
	}
	if font != nil {
		note.SetFont(font)
	}
	note.SetTextColor(ui.TextMuted)
	_ = note.SetText(text)
	return note, nil
}

// addSettingsCheckRow places a checkbox next to a regular Label so the label
// respects our dark-theme foreground colour (the native CheckBox caption is
// painted by Windows with the classic near-black text on unthemed controls).
// Clicking the label toggles the checkbox. The caption Label is returned so
// callers can re-apply its text when the UI language changes.
func addSettingsCheckRow(parent walk.Container, text string, font *walk.Font) (*walk.CheckBox, *walk.Label, error) {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return nil, nil, err
	}
	layout := walk.NewHBoxLayout()
	if err := layout.SetSpacing(8); err != nil {
		return nil, nil, err
	}
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, nil, err
	}
	if err := row.SetLayout(layout); err != nil {
		return nil, nil, err
	}

	box, err := walk.NewCheckBox(row)
	if err != nil {
		return nil, nil, err
	}
	_ = box.SetText("")
	if err := box.SetMinMaxSize(walk.Size{Width: 18, Height: 18}, walk.Size{Width: 18, Height: 18}); err != nil {
		return nil, nil, err
	}

	label, err := walk.NewLabel(row)
	if err != nil {
		return nil, nil, err
	}
	if font != nil {
		label.SetFont(font)
	}
	label.SetTextColor(ui.TextPrimary)
	_ = label.SetText(text)
	if err := label.SetAlignment(walk.AlignHNearVCenter); err != nil {
		return nil, nil, err
	}
	if err := layout.SetStretchFactor(label, 1); err != nil {
		return nil, nil, err
	}

	label.MouseDown().Attach(func(_, _ int, _ walk.MouseButton) {
		box.SetChecked(!box.Checked())
	})

	return box, label, nil
}

func addSettingsLineEditRow(parent walk.Container, labelText, value string, inputBrush *walk.SolidColorBrush, rowBrush walk.Brush) (*settingsFieldRow, error) {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	if rowBrush != nil {
		row.SetBackground(rowBrush)
	}
	layout := walk.NewHBoxLayout()
	if err := layout.SetSpacing(10); err != nil {
		return nil, err
	}
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := row.SetLayout(layout); err != nil {
		return nil, err
	}

	label, err := walk.NewLabel(row)
	if err != nil {
		return nil, err
	}
	label.SetTextColor(ui.TextPrimary)
	_ = label.SetText(labelText)
	if err := label.SetMinMaxSize(walk.Size{Width: labelWidth, Height: 0}, walk.Size{Width: labelWidth, Height: maxFieldHeight}); err != nil {
		return nil, err
	}
	if err := label.SetAlignment(walk.AlignHNearVCenter); err != nil {
		return nil, err
	}

	edit, err := walk.NewLineEdit(row)
	if err != nil {
		return nil, err
	}
	if err := edit.SetText(value); err != nil {
		return nil, err
	}
	edit.SetTextColor(ui.InputText)
	if inputBrush != nil {
		edit.SetBackground(inputBrush)
	}
	if err := layout.SetStretchFactor(edit, 1); err != nil {
		return nil, err
	}

	return &settingsFieldRow{row: row, label: label, edit: edit}, nil
}

// addSettingsTextAreaRow adds a labelled multi-line TextEdit, used for free-form
// inputs like the pinned term glossary where users enter several lines.
func addSettingsTextAreaRow(parent walk.Container, labelText, value string, inputBrush *walk.SolidColorBrush, rowBrush walk.Brush) (*walk.Composite, *walk.Label, *walk.TextEdit, error) {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return nil, nil, nil, err
	}
	if rowBrush != nil {
		row.SetBackground(rowBrush)
	}
	layout := walk.NewHBoxLayout()
	if err := layout.SetSpacing(10); err != nil {
		return nil, nil, nil, err
	}
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, nil, nil, err
	}
	if err := row.SetLayout(layout); err != nil {
		return nil, nil, nil, err
	}

	label, err := walk.NewLabel(row)
	if err != nil {
		return nil, nil, nil, err
	}
	label.SetTextColor(ui.TextPrimary)
	_ = label.SetText(labelText)
	if err := label.SetMinMaxSize(walk.Size{Width: labelWidth, Height: 0}, walk.Size{Width: labelWidth, Height: maxFieldHeight}); err != nil {
		return nil, nil, nil, err
	}
	if err := label.SetAlignment(walk.AlignHNearVNear); err != nil {
		return nil, nil, nil, err
	}

	// WS_VSCROLL | ES_MULTILINE | ES_WANTRETURN | ES_AUTOVSCROLL
	const (
		wsVScroll     uint32 = 0x00200000
		esMultiline   uint32 = 0x0004
		esWantReturn  uint32 = 0x1000
		esAutoVScroll uint32 = 0x0040
	)
	edit, err := walk.NewTextEditWithStyle(row, wsVScroll|esMultiline|esWantReturn|esAutoVScroll)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := edit.SetText(value); err != nil {
		return nil, nil, nil, err
	}
	edit.SetTextColor(ui.InputText)
	if inputBrush != nil {
		edit.SetBackground(inputBrush)
	}
	if err := edit.SetMinMaxSize(walk.Size{Width: 0, Height: 96}, walk.Size{Width: 0, Height: 180}); err != nil {
		return nil, nil, nil, err
	}
	if err := layout.SetStretchFactor(edit, 1); err != nil {
		return nil, nil, nil, err
	}

	return row, label, edit, nil
}

func addSettingsComboBoxRow(parent walk.Container, labelText string, options []string, value string, inputBrush *walk.SolidColorBrush, rowBrush walk.Brush) (*walk.ComboBox, *walk.Label, error) {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return nil, nil, err
	}
	if rowBrush != nil {
		row.SetBackground(rowBrush)
	}
	layout := walk.NewHBoxLayout()
	if err := layout.SetSpacing(10); err != nil {
		return nil, nil, err
	}
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, nil, err
	}
	if err := row.SetLayout(layout); err != nil {
		return nil, nil, err
	}

	label, err := walk.NewLabel(row)
	if err != nil {
		return nil, nil, err
	}
	label.SetTextColor(ui.TextPrimary)
	_ = label.SetText(labelText)
	if err := label.SetMinMaxSize(walk.Size{Width: labelWidth, Height: 0}, walk.Size{Width: labelWidth, Height: maxFieldHeight}); err != nil {
		return nil, nil, err
	}
	if err := label.SetAlignment(walk.AlignHNearVCenter); err != nil {
		return nil, nil, err
	}

	box, err := walk.NewComboBox(row)
	if err != nil {
		return nil, nil, err
	}
	if err := box.SetModel(options); err != nil {
		return nil, nil, err
	}
	if err := box.SetCurrentIndex(indexOfString(options, value)); err != nil {
		return nil, nil, err
	}
	if inputBrush != nil {
		box.SetBackground(inputBrush)
	}
	ui.ApplyNativeDarkTheme(box)
	if err := layout.SetStretchFactor(box, 1); err != nil {
		return nil, nil, err
	}

	return box, label, nil
}

func buildTargetLanguageOptions(currentValue string) []string {
	options := translator.TargetLanguageOptions()
	current := translator.CanonicalTargetLanguage(currentValue)
	if current == "" || indexOfString(options, current) >= 0 {
		return options
	}

	return append([]string{current}, options...)
}

func indexOfString(options []string, value string) int {
	normalized := strings.TrimSpace(value)
	for index, option := range options {
		if strings.EqualFold(strings.TrimSpace(option), normalized) {
			return index
		}
	}

	return 0
}

func addSettingsProviderRow(parent walk.Container, options []string, value string, rowBrush walk.Brush) ([]*walk.PushButton, error) {
	row, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	if rowBrush != nil {
		row.SetBackground(rowBrush)
	}
	layout := walk.NewHBoxLayout()
	if err := layout.SetSpacing(8); err != nil {
		return nil, err
	}
	if err := layout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := row.SetLayout(layout); err != nil {
		return nil, err
	}

	label, err := walk.NewLabel(row)
	if err != nil {
		return nil, err
	}
	label.SetTextColor(ui.TextPrimary)
	_ = label.SetText("Provider")
	if err := label.SetMinMaxSize(walk.Size{Width: labelWidth, Height: 0}, walk.Size{Width: labelWidth, Height: maxFieldHeight}); err != nil {
		return nil, err
	}
	if err := label.SetAlignment(walk.AlignHNearVCenter); err != nil {
		return nil, err
	}

	buttonsHost, err := walk.NewComposite(row)
	if err != nil {
		return nil, err
	}
	if rowBrush != nil {
		buttonsHost.SetBackground(rowBrush)
	}
	if err := layout.SetStretchFactor(buttonsHost, 1); err != nil {
		return nil, err
	}
	buttonsLayout := walk.NewHBoxLayout()
	if err := buttonsLayout.SetSpacing(8); err != nil {
		return nil, err
	}
	if err := buttonsLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := buttonsHost.SetLayout(buttonsLayout); err != nil {
		return nil, err
	}

	buttons := make([]*walk.PushButton, 0, len(options))
	for _, option := range options {
		button, err := walk.NewPushButton(buttonsHost)
		if err != nil {
			return nil, err
		}
		_ = button.SetText(option)
		if err := button.SetMinMaxSize(walk.Size{Width: 124, Height: 34}, walk.Size{Width: 16777215, Height: 34}); err != nil {
			return nil, err
		}
		buttons = append(buttons, button)
	}

	return buttons, nil
}

func applySettingsProviderDefaults(previousProvider string, nextProvider string, baseURLEdit *walk.LineEdit, modelEdit *walk.LineEdit) {
	replaceSettingsIfDefault(baseURLEdit, translator.DefaultBaseURL(previousProvider), translator.DefaultBaseURL(nextProvider))

	currentModel := strings.TrimSpace(modelEdit.Text())
	oldDefaultModel := translator.DefaultModel(previousProvider)
	newDefaultModel := translator.DefaultModel(nextProvider)
	if !translator.UsesModel(nextProvider) {
		if currentModel == "" || currentModel == oldDefaultModel {
			_ = modelEdit.SetText("")
		}
		return
	}

	if currentModel == "" || currentModel == oldDefaultModel {
		_ = modelEdit.SetText(newDefaultModel)
	}
}

func replaceSettingsIfDefault(edit *walk.LineEdit, oldDefault string, newDefault string) {
	current := strings.TrimSpace(edit.Text())
	if current == "" || current == oldDefault {
		_ = edit.SetText(newDefault)
	}
}

func collectPanelSettings(
	base settings.Values,
	provider string,
	apiKey string,
	baseURL string,
	model string,
	translationContext string,
	glossary string,
	sourceLanguage string,
	targetLanguage string,
	pollMs string,
	timeoutMs string,
	frequencyMs string,
	processName string,
	windowClass string,
	automationID string,
	fontSize string,
	textColor string,
	alternateTextColor string,
	alternateLineColors bool,
	alwaysOnTop bool,
	clickThrough bool,
	wordByWord bool,
	showOriginal bool,
	uiLanguage string,
	streamingEnabled bool,
) (settings.Values, string) {
	updated := base
	updated.Provider = translator.NormalizeProvider(provider)
	updated.APIKey = strings.TrimSpace(apiKey)
	updated.BaseURL = strings.TrimSpace(baseURL)
	updated.Model = strings.TrimSpace(model)
	updated.TranslationContext = strings.TrimSpace(translationContext)
	updated.Glossary = strings.TrimSpace(glossary)
	updated.SourceLanguage = strings.TrimSpace(sourceLanguage)
	updated.TargetLanguage = translator.CanonicalTargetLanguage(targetLanguage)
	updated.CaptionProcessName = strings.TrimSpace(processName)
	updated.CaptionWindowClass = strings.TrimSpace(windowClass)
	updated.CaptionAutomationID = strings.TrimSpace(automationID)
	updated.AlternateLineColors = alternateLineColors
	updated.AlwaysOnTop = alwaysOnTop
	updated.ClickThrough = clickThrough
	updated.WordByWord = wordByWord
	updated.ShowOriginal = showOriginal
	updated.UILanguage = i18n.Normalize(uiLanguage)
	updated.StreamingEnabled = streamingEnabled

	// Validation messages are rendered in the UI language the user just
	// picked in the dropdown so feedback is consistent with the rest of the
	// panel (even before Save has been clicked).
	lang := updated.UILanguage

	parsedPollMs, err := strconv.Atoi(strings.TrimSpace(pollMs))
	if err != nil || parsedPollMs <= 0 {
		return base, i18n.T(lang, "settings.error.pollMs")
	}
	updated.CaptionPollMs = parsedPollMs

	parsedTimeoutMs, err := strconv.Atoi(strings.TrimSpace(timeoutMs))
	if err != nil || parsedTimeoutMs <= 0 {
		return base, i18n.T(lang, "settings.error.timeoutMs")
	}
	updated.RequestTimeoutMs = parsedTimeoutMs

	parsedFrequencyMs, err := strconv.Atoi(strings.TrimSpace(frequencyMs))
	if err != nil || parsedFrequencyMs <= 0 {
		return base, i18n.T(lang, "settings.error.frequencyMs")
	}
	updated.RequestFrequencyMs = parsedFrequencyMs

	parsedFontSize, err := strconv.Atoi(strings.TrimSpace(fontSize))
	if err != nil || parsedFontSize <= 0 {
		return base, i18n.T(lang, "settings.error.fontSize")
	}
	updated.FontSize = parsedFontSize

	normalizedTextColor := settings.NormalizeHexColor(textColor, "")
	if normalizedTextColor == "" {
		return base, i18n.T(lang, "settings.error.primaryColor")
	}
	updated.TextColor = normalizedTextColor

	normalizedAlternateColor := settings.NormalizeHexColor(alternateTextColor, "")
	if normalizedAlternateColor == "" {
		return base, i18n.T(lang, "settings.error.alternateColor")
	}
	updated.AlternateTextColor = normalizedAlternateColor

	updated = settings.Sanitize(updated)
	return updated, ""
}
