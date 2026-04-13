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
	sectionEyebrow.SetTextColor(ui.AccentSoft)
	_ = sectionEyebrow.SetText("QUICK SETUP")

	intro, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
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
	if err := translationTabButton.SetMinMaxSize(walk.Size{Width: 132, Height: 34}, walk.Size{Width: 16777215, Height: 34}); err != nil {
		return nil, err
	}

	captionsTabButton, err := walk.NewPushButton(tabRow)
	if err != nil {
		return nil, err
	}
	if err := captionsTabButton.SetMinMaxSize(walk.Size{Width: 132, Height: 34}, walk.Size{Width: 16777215, Height: 34}); err != nil {
		return nil, err
	}

	appearanceTabButton, err := walk.NewPushButton(tabRow)
	if err != nil {
		return nil, err
	}
	if err := appearanceTabButton.SetMinMaxSize(walk.Size{Width: 132, Height: 34}, walk.Size{Width: 16777215, Height: 34}); err != nil {
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
	translationGroup, err := newSettingsSection(translationPage, "Translation provider", sectionBrush)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(translationGroup, "Choose the backend, then use Test Connection before you close the panel."); err != nil {
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
	panel.contextRow, err = addSettingsLineEditRow(translationGroup, "Translation context", current.TranslationContext, inputBrush, sectionBrush)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(translationGroup, "Optional: used by Ollama and LM Studio as additional context in the translation prompt."); err != nil {
		return nil, err
	}

	languagesGroup, err := newSettingsSection(translationPage, "Languages", sectionBrush)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(languagesGroup, "Leave source on auto unless you have a stable single-language input."); err != nil {
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
	windowGroup, err := newSettingsSection(captionsPage, "Source window", sectionBrush)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(windowGroup, "Defaults match the current Windows 11 Live Captions window. Change these only if Microsoft changes the UI element names."); err != nil {
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

	timingGroup, err := newSettingsSection(captionsPage, "Timing and latency", sectionBrush)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(timingGroup, "Lower polling feels snappier but can create more churn when captions change very quickly."); err != nil {
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
	_ = panel.wordByWordBox.SetText("Use fast refresh mode")
	if _, err := addSettingsGroupNote(timingGroup, "When enabled, uses a faster 50ms refresh rate instead of the Request frequency ms value above."); err != nil {
		return nil, err
	}

	appearancePage, err := newSettingsPage(pagesHost, panelBrush)
	if err != nil {
		return nil, err
	}
	previewGroup, err := newSettingsSection(appearancePage, "Preview", sectionBrush)
	if err != nil {
		return nil, err
	}
	if _, err := addSettingsGroupNote(previewGroup, "Font size updates immediately. Use #RRGGBB for line colors, then enable alternating colors if adjacent lines should swap colors."); err != nil {
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
	_ = panel.alwaysOnTopBox.SetText("Keep window always on top")
	panel.clickThroughBox, err = walk.NewCheckBox(previewGroup)
	if err != nil {
		return nil, err
	}
	if sectionBrush != nil {
		panel.clickThroughBox.SetBackground(sectionBrush)
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
	panel.statusLabel.SetTextColor(ui.Error)
	if err := footerLayout.SetStretchFactor(panel.statusLabel, 1); err != nil {
		return nil, err
	}

	applyButton, err := walk.NewPushButton(footer)
	if err != nil {
		return nil, err
	}
	_ = applyButton.SetText("Apply")
	if err := applyButton.SetMinMaxSize(walk.Size{Width: 124, Height: 40}, walk.Size{Width: 164, Height: 40}); err != nil {
		return nil, err
	}

	testButton, err := walk.NewPushButton(footer)
	if err != nil {
		return nil, err
	}
	_ = testButton.SetText("Test Connection")
	if err := testButton.SetMinMaxSize(walk.Size{Width: 156, Height: 40}, walk.Size{Width: 196, Height: 40}); err != nil {
		return nil, err
	}

	cancelButton, err := walk.NewPushButton(footer)
	if err != nil {
		return nil, err
	}
	_ = cancelButton.SetText("Close")
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
	showTab := func(index int) {
		for i, page := range tabPages {
			page.SetVisible(i == index)
			label := tabTitles[i]
			if i == index {
				label = "[" + label + "]"
			}
			_ = tabButtons[i].SetText(label)
		}
	}

	translationTabButton.Clicked().Attach(func() { showTab(0) })
	captionsTabButton.Clicked().Attach(func() { showTab(1) })
	appearanceTabButton.Clicked().Attach(func() { showTab(2) })

	for _, button := range panel.providerButtons {
		providerName := strings.Trim(button.Text(), "[]")
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
}

func (p *settingsPanel) selectedTargetLanguage() string {
	if p.targetLangBox == nil {
		return "English"
	}

	return translator.CanonicalTargetLanguage(p.targetLangBox.Text())
}

func (p *settingsPanel) updateProviderButtons(provider string) {
	normalized := translator.NormalizeProvider(provider)
	for _, button := range p.providerButtons {
		label := button.Text()
		label = strings.TrimPrefix(label, "[")
		label = strings.TrimSuffix(label, "]")
		if translator.NormalizeProvider(label) == normalized {
			label = "[" + label + "]"
		}
		_ = button.SetText(label)
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

func newSettingsSection(parent walk.Container, title string, background walk.Brush) (*walk.Composite, error) {
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
	heading.SetTextColor(ui.AccentSoft)
	_ = heading.SetText(title)
	return group, nil
}

func addSettingsGroupNote(parent walk.Container, text string) (*walk.Label, error) {
	note, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
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
