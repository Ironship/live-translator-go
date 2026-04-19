//go:build windows

package app

import (
	"context"
	"strconv"
	"strings"
	"time"

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
	statusLabel       *walk.Label
	selectedProvider  string
	base              settings.Values
	onSave            func(settings.Values) error
	onCancel          func()
	providerLabels    []string
	tabFont           *walk.Font
	tabFontSelected   *walk.Font
}

func newSettingsPanel(parent walk.Container, current settings.Values, onSave func(settings.Values) error, onCancel func()) (*settingsPanel, error) {
	panel := &settingsPanel{onSave: onSave, onCancel: onCancel}

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
	_ = sectionEyebrow.SetText("QUICK SETUP")

	intro, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
	}
	if introFont != nil {
		intro.SetFont(introFont)
	}
	intro.SetTextColor(ui.TextSecondary)
	_ = intro.SetText("Provider, source window, and preview options are grouped into focused tabs so you can change one thing at a time without hunting through the whole form.")

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
	translationGroup, err := newSettingsSection(translationPage, "Translation provider", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(translationGroup, "Choose the backend, then use Test Connection before you close the panel.", bodyFont); err != nil {
		return nil, err
	}
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
	panel.contextRow, err = addSettingsLineEditRow(translationGroup, "Translation context (optional)", current.TranslationContext, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.contextNote, err = addSettingsGroupNote(translationGroup, "Optional: used by Ollama and LM Studio as additional context in the translation prompt.", bodyFont)
	if err != nil {
		return nil, err
	}

	languagesGroup, err := newSettingsSection(translationPage, "Languages", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(languagesGroup, "Leave source on auto unless you have a stable single-language input.", bodyFont); err != nil {
		return nil, err
	}
	panel.sourceLangRow, err = addSettingsLineEditRow(languagesGroup, "Source language", current.SourceLanguage, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	targetLanguageOptions := buildTargetLanguageOptions(current.TargetLanguage)
	panel.targetLangBox, err = addSettingsComboBoxRow(languagesGroup, "Target language", targetLanguageOptions, translator.CanonicalTargetLanguage(current.TargetLanguage), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}

	captionsPage, err := newSettingsPage(pagesHost, panelBrush)
	if err != nil {
		return nil, err
	}
	windowGroup, err := newSettingsSection(captionsPage, "Source window", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(windowGroup, "Defaults match the current Windows 11 Live Captions window. Change these only if Microsoft changes the UI element names.", bodyFont); err != nil {
		return nil, err
	}
	panel.processRow, err = addSettingsLineEditRow(windowGroup, "Process name", current.CaptionProcessName, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.windowClassRow, err = addSettingsLineEditRow(windowGroup, "Window class", current.CaptionWindowClass, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.automationIDRow, err = addSettingsLineEditRow(windowGroup, "Automation id", current.CaptionAutomationID, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}

	timingGroup, err := newSettingsSection(captionsPage, "Timing and latency", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(timingGroup, "Lower polling feels snappier but can create more churn when captions change very quickly.", bodyFont); err != nil {
		return nil, err
	}
	panel.pollMsRow, err = addSettingsLineEditRow(timingGroup, "Caption poll ms", strconv.Itoa(current.CaptionPollMs), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.timeoutMsRow, err = addSettingsLineEditRow(timingGroup, "Request timeout ms", strconv.Itoa(current.RequestTimeoutMs), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.frequencyMsRow, err = addSettingsLineEditRow(timingGroup, "Request frequency ms", strconv.Itoa(current.RequestFrequencyMs), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.wordByWordBox, err = walk.NewCheckBox(timingGroup)
	if err != nil {
		return nil, err
	}
	if sectionBrush != nil {
		panel.wordByWordBox.SetBackground(sectionBrush)
	}
	if bodyFont != nil {
		panel.wordByWordBox.SetFont(bodyFont)
	}
	_ = panel.wordByWordBox.SetText("Translate word by word (like Live Captions)")
	if _, err := addSettingsGroupNote(timingGroup, "When enabled, translations start immediately on each caption change. Request frequency ms is ignored.", bodyFont); err != nil {
		return nil, err
	}

	appearancePage, err := newSettingsPage(pagesHost, panelBrush)
	if err != nil {
		return nil, err
	}
	previewGroup, err := newSettingsSection(appearancePage, "Preview", sectionBrush, headingFont, bodyFont)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(previewGroup, "Font size updates immediately. Use #RRGGBB for line colors, then enable alternating colors if adjacent lines should swap colors.", bodyFont); err != nil {
		return nil, err
	}
	panel.fontSizeRow, err = addSettingsLineEditRow(previewGroup, "Font size", strconv.Itoa(current.FontSize), inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.textColorRow, err = addSettingsLineEditRow(previewGroup, "Primary line color", current.TextColor, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.alternateLinesBox, err = walk.NewCheckBox(previewGroup)
	if err != nil {
		return nil, err
	}
	if sectionBrush != nil {
		panel.alternateLinesBox.SetBackground(sectionBrush)
	}
	if bodyFont != nil {
		panel.alternateLinesBox.SetFont(bodyFont)
	}
	_ = panel.alternateLinesBox.SetText("Use alternating line colors")
	panel.alternateColorRow, err = addSettingsLineEditRow(previewGroup, "Alternate line color", current.AlternateTextColor, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	panel.alwaysOnTopBox, err = walk.NewCheckBox(previewGroup)
	if err != nil {
		return nil, err
	}
	if sectionBrush != nil {
		panel.alwaysOnTopBox.SetBackground(sectionBrush)
	}
	if bodyFont != nil {
		panel.alwaysOnTopBox.SetFont(bodyFont)
	}
	_ = panel.alwaysOnTopBox.SetText("Keep window always on top")
	panel.clickThroughBox, err = walk.NewCheckBox(previewGroup)
	if err != nil {
		return nil, err
	}
	if sectionBrush != nil {
		panel.clickThroughBox.SetBackground(sectionBrush)
	}
	if bodyFont != nil {
		panel.clickThroughBox.SetFont(bodyFont)
	}
	_ = panel.clickThroughBox.SetText("Allow click-through in compact mode")
	panel.alternateLinesBox.CheckedChanged().Attach(func() {
		panel.updateAppearanceRows()
	})

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
	_ = applyButton.SetText(ui.SymbolSave + "Save")
	_ = applyButton.SetToolTipText("Save settings and apply them immediately")
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
	_ = testButton.SetText(ui.SymbolTest + "Test Connection")
	_ = testButton.SetToolTipText("Send a small test request to the selected provider")
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
	_ = cancelButton.SetText(ui.SymbolCancel + "Close")
	_ = cancelButton.SetToolTipText("Discard unsaved edits and close the settings panel")
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
		applyButton,
		testButton,
		cancelButton,
	)
	for _, button := range panel.providerButtons {
		ui.ApplyNativeDarkTheme(button)
	}

	tabTitles := []string{"Translation", "Live Captions", "Appearance"}
	tabButtons := []*walk.PushButton{translationTabButton, captionsTabButton, appearanceTabButton}
	tabPages := []*walk.Composite{translationPage, captionsPage, appearancePage}
	panel.tabFont = tabFont
	panel.tabFontSelected = tabFontSelected
	showTab := func(index int) {
		for i, page := range tabPages {
			page.SetVisible(i == index)
			if i == index {
				_ = tabButtons[i].SetText(ui.BulletSelected + tabTitles[i])
				if tabFontSelected != nil {
					tabButtons[i].SetFont(tabFontSelected)
				}
			} else {
				_ = tabButtons[i].SetText(ui.BulletIdle + tabTitles[i])
				if tabFont != nil {
					tabButtons[i].SetFont(tabFont)
				}
			}
		}
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

		panel.showInfo("Testing provider connection...")
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
	p.updateProviderRows(values.Provider)
	p.updateAppearanceRows()
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
}

func (p *settingsPanel) selectedTargetLanguage() string {
	if p.targetLangBox == nil {
		return "English"
	}

	return translator.CanonicalTargetLanguage(p.targetLangBox.Text())
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

func newSettingsSection(parent walk.Container, title string, background walk.Brush, headingFont, titleFont *walk.Font) (*walk.Composite, error) {
	group, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	if background != nil {
		group.SetBackground(background)
	}
	layout := walk.NewVBoxLayout()
	if err := layout.SetSpacing(12); err != nil {
		return nil, err
	}
	if err := layout.SetMargins(walk.Margins{HNear: 18, VNear: 18, HFar: 18, VFar: 18}); err != nil {
		return nil, err
	}
	if err := group.SetLayout(layout); err != nil {
		return nil, err
	}
	heading, err := walk.NewLabel(group)
	if err != nil {
		return nil, err
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
	return group, nil
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

func addSettingsComboBoxRow(parent walk.Container, labelText string, options []string, value string, inputBrush *walk.SolidColorBrush, rowBrush walk.Brush) (*walk.ComboBox, error) {
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

	box, err := walk.NewComboBox(row)
	if err != nil {
		return nil, err
	}
	if err := box.SetModel(options); err != nil {
		return nil, err
	}
	if err := box.SetCurrentIndex(indexOfString(options, value)); err != nil {
		return nil, err
	}
	if inputBrush != nil {
		box.SetBackground(inputBrush)
	}
	ui.ApplyNativeDarkTheme(box)
	if err := layout.SetStretchFactor(box, 1); err != nil {
		return nil, err
	}

	return box, nil
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
) (settings.Values, string) {
	updated := base
	updated.Provider = translator.NormalizeProvider(provider)
	updated.APIKey = strings.TrimSpace(apiKey)
	updated.BaseURL = strings.TrimSpace(baseURL)
	updated.Model = strings.TrimSpace(model)
	updated.TranslationContext = strings.TrimSpace(translationContext)
	updated.SourceLanguage = strings.TrimSpace(sourceLanguage)
	updated.TargetLanguage = translator.CanonicalTargetLanguage(targetLanguage)
	updated.CaptionProcessName = strings.TrimSpace(processName)
	updated.CaptionWindowClass = strings.TrimSpace(windowClass)
	updated.CaptionAutomationID = strings.TrimSpace(automationID)
	updated.AlternateLineColors = alternateLineColors
	updated.AlwaysOnTop = alwaysOnTop
	updated.ClickThrough = clickThrough
	updated.WordByWord = wordByWord

	parsedPollMs, err := strconv.Atoi(strings.TrimSpace(pollMs))
	if err != nil || parsedPollMs <= 0 {
		return base, "Caption poll ms musi byc dodatnia liczba calkowita."
	}
	updated.CaptionPollMs = parsedPollMs

	parsedTimeoutMs, err := strconv.Atoi(strings.TrimSpace(timeoutMs))
	if err != nil || parsedTimeoutMs <= 0 {
		return base, "Request timeout ms musi byc dodatnia liczba calkowita."
	}
	updated.RequestTimeoutMs = parsedTimeoutMs

	parsedFrequencyMs, err := strconv.Atoi(strings.TrimSpace(frequencyMs))
	if err != nil || parsedFrequencyMs <= 0 {
		return base, "Request frequency ms musi byc dodatnia liczba calkowita."
	}
	updated.RequestFrequencyMs = parsedFrequencyMs

	parsedFontSize, err := strconv.Atoi(strings.TrimSpace(fontSize))
	if err != nil || parsedFontSize <= 0 {
		return base, "Font size musi byc dodatnia liczba calkowita."
	}
	updated.FontSize = parsedFontSize

	normalizedTextColor := settings.NormalizeHexColor(textColor, "")
	if normalizedTextColor == "" {
		return base, "Primary line color musi miec format #RRGGBB."
	}
	updated.TextColor = normalizedTextColor

	normalizedAlternateColor := settings.NormalizeHexColor(alternateTextColor, "")
	if normalizedAlternateColor == "" {
		return base, "Alternate line color musi miec format #RRGGBB."
	}
	updated.AlternateTextColor = normalizedAlternateColor

	updated = settings.Sanitize(updated)
	return updated, ""
}
