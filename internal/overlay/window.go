//go:build windows

package overlay

import (
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"live-translator-go/internal/ui"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

type Color struct {
	R byte
	G byte
	B byte
}

type Config struct {
	InitialText         string
	FontFamily          string
	FontSize            int
	Height              int
	HorizontalMargin    int
	BottomOffset        int
	BackgroundColor     Color
	TextColor           Color
	AlternateTextColor  Color
	AlternateLineColors bool
	AlwaysOnTop         bool
	ClickThrough        bool
}

type Window struct {
	mainWindow          *walk.MainWindow
	shell               *walk.Composite
	rootLayout          *walk.BoxLayout
	headerCard          *walk.Composite
	previewCard         *walk.Composite
	previewPanel        *walk.GradientComposite
	previewHeader       *walk.GradientComposite
	previewLayout       *walk.BoxLayout
	previewStage        *walk.GradientComposite
	previewStageLay     *walk.BoxLayout
	previewTitle        *walk.Label
	focusHint           *walk.Label
	titleLabel          *walk.Label
	statusLabel         *walk.Label
	previewSurface      *previewSurface
	openCaptions        *walk.PushButton
	speechPanel         *walk.PushButton
	settings            *walk.PushButton
	alwaysOnTop         *walk.PushButton
	clearButton         *walk.PushButton
	focus               *walk.PushButton
	exit                *walk.PushButton
	settingsCard        *walk.Composite
	settingsHost        *walk.Composite
	shellBrush          *walk.SolidColorBrush
	previewBrush        *walk.SolidColorBrush
	previewStageB       *walk.SolidColorBrush
	focusShellBrush     *walk.SolidColorBrush
	focusPreviewB       *walk.SolidColorBrush
	focusStageB         *walk.SolidColorBrush
	lastText            string
	captionHistory      []previewLine
	lastCaptionSnapshot []string
	currentConfig       Config
	collapsedBounds     walk.Rectangle
	expandedBounds      walk.Rectangle
	focusBounds         walk.Rectangle
	applyingBounds      bool
	settingsVisible     bool
	focusMode           bool
	alwaysOnTopEnabled  bool
	increaseFontSize    func()
	decreaseFontSize    func()
}

const (
	minCaptionHistoryLines   = 8
	maxCaptionHistoryLines   = 36
	historyLineHeadroom      = 4
	minEstimatedPreviewLineH = 26
	layeredWindowAlphaFlag   = 0x2
	normalWindowAlpha        = 255
	focusWindowAlpha         = 224
)

var (
	setLayeredWindowAttributesProc = syscall.NewLazyDLL("user32.dll").NewProc("SetLayeredWindowAttributes")
	dwmSetWindowAttributeProc      = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	sentenceBreakPattern           = regexp.MustCompile(`([.!?]+[)"'\]]*)(\s+)`)
	captionTokenNormalizer         = strings.NewReplacer(
		"ą", "a",
		"ć", "c",
		"ę", "e",
		"ł", "l",
		"ń", "n",
		"ó", "o",
		"ś", "s",
		"ź", "z",
		"ż", "z",
	)
)

func New(config Config) (*Window, error) {
	config = withDefaults(config)

	mainWindow, err := walk.NewMainWindow()
	if err != nil {
		return nil, err
	}
	if err := mainWindow.SetTitle("Live Translator"); err != nil {
		return nil, err
	}
	if err := mainWindow.SetMinMaxSize(walk.Size{720, 200}, walk.Size{2200, 1400}); err != nil {
		return nil, err
	}

	mainLayout := walk.NewVBoxLayout()
	if err := mainLayout.SetSpacing(0); err != nil {
		return nil, err
	}
	if err := mainLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := mainWindow.SetLayout(mainLayout); err != nil {
		return nil, err
	}
	applyDarkTitleBar(mainWindow.Handle())

	shellBrush, err := walk.NewSolidColorBrush(ui.AppBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(shellBrush)
	mainWindow.SetBackground(shellBrush)

	shell, err := walk.NewComposite(mainWindow)
	if err != nil {
		return nil, err
	}
	shell.SetBackground(shellBrush)
	if err := mainLayout.SetStretchFactor(shell, 1); err != nil {
		return nil, err
	}

	rootLayout := walk.NewVBoxLayout()
	if err := rootLayout.SetSpacing(16); err != nil {
		return nil, err
	}
	if err := rootLayout.SetMargins(walk.Margins{HNear: 18, VNear: 18, HFar: 18, VFar: 18}); err != nil {
		return nil, err
	}
	if err := shell.SetLayout(rootLayout); err != nil {
		return nil, err
	}

	headerBrush, err := walk.NewSolidColorBrush(ui.CardBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(headerBrush)

	settingsBrush, err := walk.NewSolidColorBrush(ui.PanelBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(settingsBrush)

	focusShellBrush, err := walk.NewSolidColorBrush(ui.FocusBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(focusShellBrush)

	focusPreviewBrush, err := walk.NewSolidColorBrush(ui.FocusPanelBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(focusPreviewBrush)

	focusStageBrush, err := walk.NewSolidColorBrush(ui.FocusStageBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(focusStageBrush)

	eyebrowFont, err := walk.NewFont("Bahnschrift SemiCondensed", 10, walk.FontBold)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(eyebrowFont)

	titleFont, err := walk.NewFont("Segoe UI Semibold", 22, walk.FontBold)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(titleFont)

	statusFont, err := walk.NewFont("Segoe UI", 10, 0)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(statusFont)

	bodyFont, err := walk.NewFont("Segoe UI", 11, 0)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(bodyFont)

	buttonFont, err := walk.NewFont("Segoe UI Semibold", 10, 0)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(buttonFont)

	headerCard, err := walk.NewComposite(shell)
	if err != nil {
		return nil, err
	}
	headerCard.SetBackground(headerBrush)
	headerCardLayout := walk.NewVBoxLayout()
	if err := headerCardLayout.SetSpacing(0); err != nil {
		return nil, err
	}
	if err := headerCardLayout.SetMargins(walk.Margins{HNear: 16, VNear: 14, HFar: 16, VFar: 14}); err != nil {
		return nil, err
	}
	if err := headerCard.SetLayout(headerCardLayout); err != nil {
		return nil, err
	}

	headerRow, err := walk.NewComposite(headerCard)
	if err != nil {
		return nil, err
	}
	headerLayout := walk.NewHBoxLayout()
	if err := headerLayout.SetSpacing(12); err != nil {
		return nil, err
	}
	if err := headerLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := headerRow.SetLayout(headerLayout); err != nil {
		return nil, err
	}
	headerRow.SetBackground(headerBrush)

	brandColumn, err := walk.NewComposite(headerRow)
	if err != nil {
		return nil, err
	}
	brandLayout := walk.NewVBoxLayout()
	if err := brandLayout.SetSpacing(3); err != nil {
		return nil, err
	}
	if err := brandLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := brandColumn.SetLayout(brandLayout); err != nil {
		return nil, err
	}
	brandColumn.SetBackground(headerBrush)
	if err := headerLayout.SetStretchFactor(brandColumn, 1); err != nil {
		return nil, err
	}

	titleLabel, err := walk.NewLabel(brandColumn)
	if err != nil {
		return nil, err
	}
	titleLabel.SetFont(titleFont)
	titleLabel.SetTextColor(ui.TextPrimary)
	_ = titleLabel.SetText("Live Translator")

	statusLabel, err := walk.NewLabel(brandColumn)
	if err != nil {
		return nil, err
	}
	statusLabel.SetFont(statusFont)
	statusLabel.SetTextColor(ui.TextSecondary)
	_ = statusLabel.SetText("Status: waiting for Live Captions")

	buttonRow, err := walk.NewComposite(headerRow)
	if err != nil {
		return nil, err
	}
	buttonLayout := walk.NewHBoxLayout()
	if err := buttonLayout.SetSpacing(8); err != nil {
		return nil, err
	}
	if err := buttonLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := buttonRow.SetLayout(buttonLayout); err != nil {
		return nil, err
	}
	buttonRow.SetBackground(headerBrush)

	openCaptionsButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	openCaptionsButton.SetFont(buttonFont)
	_ = openCaptionsButton.SetText("Start")
	if err := openCaptionsButton.SetMinMaxSize(walk.Size{86, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}

	speechPanelButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	speechPanelButton.SetFont(buttonFont)
	_ = speechPanelButton.SetText("Speech")
	if err := speechPanelButton.SetMinMaxSize(walk.Size{78, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}

	settingsButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	settingsButton.SetFont(buttonFont)
	_ = settingsButton.SetText("Settings")
	if err := settingsButton.SetMinMaxSize(walk.Size{88, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}

	alwaysOnTopButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	alwaysOnTopButton.SetFont(buttonFont)
	_ = alwaysOnTopButton.SetText("On Top: On")
	if err := alwaysOnTopButton.SetMinMaxSize(walk.Size{102, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}

	clearButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	clearButton.SetFont(buttonFont)
	_ = clearButton.SetText("Clear")
	if err := clearButton.SetMinMaxSize(walk.Size{68, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}

	focusButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	focusButton.SetFont(buttonFont)
	_ = focusButton.SetText("Focus")
	if err := focusButton.SetMinMaxSize(walk.Size{72, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}

	exitButton, err := walk.NewPushButton(buttonRow)
	if err != nil {
		return nil, err
	}
	exitButton.SetFont(buttonFont)
	_ = exitButton.SetText("Exit")
	if err := exitButton.SetMinMaxSize(walk.Size{68, 34}, walk.Size{16777215, 34}); err != nil {
		return nil, err
	}
	ui.ApplyNativeDarkTheme(
		openCaptionsButton,
		speechPanelButton,
		settingsButton,
		alwaysOnTopButton,
		clearButton,
		focusButton,
		exitButton,
	)

	previewCard, err := walk.NewComposite(shell)
	if err != nil {
		return nil, err
	}
	if err := rootLayout.SetStretchFactor(previewCard, 1); err != nil {
		return nil, err
	}
	previewCardLayout := walk.NewVBoxLayout()
	if err := previewCardLayout.SetSpacing(0); err != nil {
		return nil, err
	}
	if err := previewCardLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := previewCard.SetLayout(previewCardLayout); err != nil {
		return nil, err
	}
	previewLayout := walk.NewVBoxLayout()
	if err := previewLayout.SetSpacing(10); err != nil {
		return nil, err
	}
	if err := previewLayout.SetMargins(walk.Margins{HNear: 16, VNear: 16, HFar: 16, VFar: 16}); err != nil {
		return nil, err
	}
	previewCard.SetBackground(shellBrush)

	previewPanel, err := walk.NewGradientComposite(previewCard)
	if err != nil {
		return nil, err
	}
	if err := previewCardLayout.SetStretchFactor(previewPanel, 1); err != nil {
		return nil, err
	}
	if err := previewPanel.SetLayout(previewLayout); err != nil {
		return nil, err
	}
	if err := setGradientCompositeColor(previewPanel, ui.PreviewBackground); err != nil {
		return nil, err
	}
	previewBrush, err := walk.NewSolidColorBrush(ui.PreviewBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(previewBrush)

	previewStageBrush, err := walk.NewSolidColorBrush(ui.PreviewStageBackground)
	if err != nil {
		return nil, err
	}
	mainWindow.AddDisposable(previewStageBrush)

	previewHeader, err := walk.NewGradientComposite(previewPanel)
	if err != nil {
		return nil, err
	}
	previewHeaderLayout := walk.NewHBoxLayout()
	if err := previewHeaderLayout.SetSpacing(0); err != nil {
		return nil, err
	}
	if err := previewHeaderLayout.SetMargins(walk.Margins{}); err != nil {
		return nil, err
	}
	if err := previewHeader.SetLayout(previewHeaderLayout); err != nil {
		return nil, err
	}
	if err := setGradientCompositeColor(previewHeader, ui.PreviewBackground); err != nil {
		return nil, err
	}

	previewTitle, err := walk.NewLabel(previewHeader)
	if err != nil {
		return nil, err
	}
	previewTitle.SetFont(eyebrowFont)
	previewTitle.SetTextColor(ui.Accent)
	_ = previewTitle.SetText("LIVE CAPTIONS")

	if _, err := walk.NewHSpacer(previewHeader); err != nil {
		return nil, err
	}

	focusHintLabel, err := walk.NewLabel(previewHeader)
	if err != nil {
		return nil, err
	}
	focusHintLabel.SetFont(statusFont)
	focusHintLabel.SetTextColor(ui.TextMuted)
	_ = focusHintLabel.SetText("[Esc to leave focus mode]")
	focusHintLabel.SetVisible(false)

	previewStage, err := walk.NewGradientComposite(previewPanel)
	if err != nil {
		return nil, err
	}
	if err := setGradientCompositeColor(previewStage, ui.PreviewStageBackground); err != nil {
		return nil, err
	}
	if err := previewStage.SetMinMaxSize(walk.Size{0, 140}, walk.Size{16777215, 16777215}); err != nil {
		return nil, err
	}
	if err := previewLayout.SetStretchFactor(previewStage, 1); err != nil {
		return nil, err
	}
	previewStageLayout := walk.NewVBoxLayout()
	if err := previewStageLayout.SetSpacing(0); err != nil {
		return nil, err
	}
	if err := previewStageLayout.SetMargins(walk.Margins{HNear: 18, VNear: 18, HFar: 18, VFar: 18}); err != nil {
		return nil, err
	}
	if err := previewStage.SetLayout(previewStageLayout); err != nil {
		return nil, err
	}

	initialLines := splitCaptionLines(config.InitialText)
	if len(initialLines) == 0 && strings.TrimSpace(config.InitialText) != "" {
		initialLines = []string{strings.TrimSpace(config.InitialText)}
	}
	initialPreviewLines := appendPreviewTextsWithPersistentColors(nil, initialLines...)
	previewSurface, err := newPreviewSurface(
		previewStage,
		initialPreviewLines,
		walk.RGB(config.TextColor.R, config.TextColor.G, config.TextColor.B),
		walk.RGB(config.AlternateTextColor.R, config.AlternateTextColor.G, config.AlternateTextColor.B),
		config.AlternateLineColors,
		walk.RGB(17, 22, 30),
		walk.RGB(7, 10, 16),
	)
	if err != nil {
		return nil, err
	}
	if err := previewSurface.Widget().SetMinMaxSize(walk.Size{0, 96}, walk.Size{16777215, 16777215}); err != nil {
		return nil, err
	}
	if err := previewStageLayout.SetStretchFactor(previewSurface.Widget(), 1); err != nil {
		return nil, err
	}

	settingsCard, err := walk.NewComposite(shell)
	if err != nil {
		return nil, err
	}
	settingsCard.SetBackground(settingsBrush)
	settingsCardLayout := walk.NewVBoxLayout()
	if err := settingsCardLayout.SetSpacing(10); err != nil {
		return nil, err
	}
	if err := settingsCardLayout.SetMargins(walk.Margins{HNear: 18, VNear: 16, HFar: 18, VFar: 16}); err != nil {
		return nil, err
	}
	if err := settingsCard.SetLayout(settingsCardLayout); err != nil {
		return nil, err
	}
	settingsCard.SetVisible(false)

	settingsEyebrow, err := walk.NewLabel(settingsCard)
	if err != nil {
		return nil, err
	}
	settingsEyebrow.SetFont(eyebrowFont)
	settingsEyebrow.SetTextColor(ui.AccentSoft)
	_ = settingsEyebrow.SetText("SETTINGS")

	settingsIntro, err := walk.NewLabel(settingsCard)
	if err != nil {
		return nil, err
	}
	settingsIntro.SetFont(bodyFont)
	settingsIntro.SetTextColor(ui.TextSecondary)
	_ = settingsIntro.SetText("Provider, caption source, and appearance changes apply after Save.")

	settingsHost, err := walk.NewComposite(settingsCard)
	if err != nil {
		return nil, err
	}
	settingsHost.SetBackground(settingsBrush)
	if err := settingsCardLayout.SetStretchFactor(settingsHost, 1); err != nil {
		return nil, err
	}

	window := &Window{
		mainWindow:      mainWindow,
		shell:           shell,
		rootLayout:      rootLayout,
		headerCard:      headerCard,
		previewCard:     previewCard,
		previewPanel:    previewPanel,
		previewHeader:   previewHeader,
		previewLayout:   previewLayout,
		previewStage:    previewStage,
		previewStageLay: previewStageLayout,
		previewTitle:    previewTitle,
		focusHint:       focusHintLabel,
		titleLabel:      titleLabel,
		statusLabel:     statusLabel,
		previewSurface:  previewSurface,
		openCaptions:    openCaptionsButton,
		speechPanel:     speechPanelButton,
		settings:        settingsButton,
		alwaysOnTop:     alwaysOnTopButton,
		clearButton:     clearButton,
		focus:           focusButton,
		exit:            exitButton,
		settingsCard:    settingsCard,
		settingsHost:    settingsHost,
		shellBrush:      shellBrush,
		previewBrush:    previewBrush,
		previewStageB:   previewStageBrush,
		focusShellBrush: focusShellBrush,
		focusPreviewB:   focusPreviewBrush,
		focusStageB:     focusStageBrush,
		lastText:        previewLineSignature(initialPreviewLines),
		captionHistory:  append([]previewLine(nil), initialPreviewLines...),
		currentConfig:   config,
		settingsVisible: false,
	}
	window.mainWindow.Disposing().Attach(func() {
		previewSurface.Stop()
	})
	handleFocusEscape := func(key walk.Key) {
		if key == walk.KeyEscape && window.focusMode {
			_ = window.SetFocusMode(false)
		}
	}
	window.mainWindow.KeyDown().Attach(handleFocusEscape)
	previewSurface.Widget().KeyDown().Attach(handleFocusEscape)

	escapeAction := walk.NewAction()
	if err := escapeAction.SetShortcut(walk.Shortcut{Key: walk.KeyEscape}); err != nil {
		return nil, err
	}
	escapeAction.Triggered().Attach(func() {
		if window.focusMode {
			_ = window.SetFocusMode(false)
		}
	})
	if err := window.mainWindow.ShortcutActions().Add(escapeAction); err != nil {
		return nil, err
	}

	fontIncreaseKeys := []walk.Key{walk.KeyAdd, walk.KeyOEMPlus}
	for _, key := range fontIncreaseKeys {
		action := walk.NewAction()
		if err := action.SetShortcut(walk.Shortcut{Key: key}); err != nil {
			return nil, err
		}
		action.Triggered().Attach(func() {
			if window.settingsVisible {
				return
			}
			if window.increaseFontSize != nil {
				window.increaseFontSize()
			}
		})
		if err := window.mainWindow.ShortcutActions().Add(action); err != nil {
			return nil, err
		}
	}

	fontDecreaseKeys := []walk.Key{walk.KeySubtract, walk.KeyOEMMinus}
	for _, key := range fontDecreaseKeys {
		action := walk.NewAction()
		if err := action.SetShortcut(walk.Shortcut{Key: key}); err != nil {
			return nil, err
		}
		action.Triggered().Attach(func() {
			if window.settingsVisible {
				return
			}
			if window.decreaseFontSize != nil {
				window.decreaseFontSize()
			}
		})
		if err := window.mainWindow.ShortcutActions().Add(action); err != nil {
			return nil, err
		}
	}

	window.mainWindow.SizeChanged().Attach(func() {
		if window.mainWindow.IsDisposed() || window.applyingBounds {
			return
		}
		window.storeBounds(window.mainWindow.BoundsPixels())
	})

	if err := window.applyConfig(config); err != nil {
		return nil, err
	}

	mainWindow.SetVisible(true)
	window.applyTopmostState(true)
	return window, nil
}

func (w *Window) OnStarted(handler func()) {
	w.mainWindow.Starting().Attach(handler)
}

func (w *Window) OnDisposing(handler func()) {
	w.mainWindow.Disposing().Attach(handler)
}

func (w *Window) OnSettings(handler func()) {
	w.settings.Clicked().Attach(handler)
}

func (w *Window) OnOpenLiveCaptions(handler func()) {
	w.openCaptions.Clicked().Attach(handler)
}

func (w *Window) OnOpenSpeechRecognition(handler func()) {
	w.speechPanel.Clicked().Attach(handler)
}

func (w *Window) OnToggleAlwaysOnTop(handler func()) {
	w.alwaysOnTop.Clicked().Attach(handler)
}

func (w *Window) OnToggleWordByWord(handler func()) {
	// Word-by-word mode is controlled from settings.
	// Keep this hook as a no-op for compatibility with controller wiring.
}

func (w *Window) SetWordByWord(enabled bool) {
	// The overlay has no dedicated word-by-word toggle button in this branch.
	// Keep this method as a no-op to satisfy the controller contract.
}

func (w *Window) OnIncreaseFontSize(handler func()) {
	w.increaseFontSize = handler
}

func (w *Window) OnDecreaseFontSize(handler func()) {
	w.decreaseFontSize = handler
}

func (w *Window) OnToggleFocusMode(handler func()) {
	w.focus.Clicked().Attach(handler)
}

func (w *Window) OnClear(handler func()) {
	w.clearButton.Clicked().Attach(handler)
}

func (w *Window) OnExit(handler func()) {
	w.exit.Clicked().Attach(handler)
}

func (w *Window) Run() int {
	return w.mainWindow.Run()
}

func (w *Window) Close() error {
	return w.mainWindow.Close()
}

func (w *Window) Form() walk.Form {
	return w.mainWindow
}

func (w *Window) SettingsHost() walk.Container {
	return w.settingsHost
}

func (w *Window) SettingsVisible() bool {
	return w.settingsVisible
}

func (w *Window) AlwaysOnTopEnabled() bool {
	return w.alwaysOnTopEnabled
}

func (w *Window) SetSettingsVisible(visible bool) {
	if w.mainWindow.IsDisposed() {
		return
	}

	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}

		if visible {
			w.focusMode = false
		}
		w.settingsVisible = visible
		w.updateActionButtons()
		_ = w.applyConfig(w.currentConfig)
	})
}

func (w *Window) ToggleAlwaysOnTop() {
	w.SetAlwaysOnTop(!w.alwaysOnTopEnabled)
}

func (w *Window) SetAlwaysOnTop(enabled bool) {
	if w.mainWindow.IsDisposed() {
		return
	}

	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}

		w.alwaysOnTopEnabled = enabled
		w.currentConfig.AlwaysOnTop = enabled
		w.updateActionButtons()
		w.applyTopmostState(false)
	})
}

func (w *Window) FocusModeEnabled() bool {
	return w.focusMode
}

func (w *Window) ToggleFocusMode() error {
	return w.SetFocusMode(!w.focusMode)
}

func (w *Window) SetFocusMode(enabled bool) error {
	if w.mainWindow.IsDisposed() {
		return nil
	}

	var applyErr error
	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}

		if w.focusMode == enabled {
			return
		}

		w.focusMode = enabled
		if enabled {
			w.settingsVisible = false
		}
		w.updateActionButtons()

		applyErr = w.applyConfig(w.currentConfig)
		if applyErr == nil && enabled {
			applyErr = w.previewSurface.Widget().SetFocus()
		}
	})

	return applyErr
}

func (w *Window) SetStatus(value string) {
	text := strings.TrimSpace(value)
	if text == "" {
		text = "Status: waiting for Live Captions"
	}

	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}
		_ = w.statusLabel.SetText(text)
	})
}

func (w *Window) ApplyConfig(config Config) error {
	if w.mainWindow.IsDisposed() {
		return nil
	}

	var applyErr error
	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}
		applyErr = w.applyConfig(config)
	})
	return applyErr
}

func (w *Window) SetText(value string) {
	text := strings.TrimSpace(value)
	if text == "" || w.mainWindow.IsDisposed() {
		return
	}

	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}
		w.captionHistory = nil
		w.lastCaptionSnapshot = nil
		w.applyPreviewLines(appendPreviewTextsWithPersistentColors(nil, splitCaptionLines(text)...), false)
	})
}

func (w *Window) Clear() {
	if w.mainWindow.IsDisposed() {
		return
	}
	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}
		w.captionHistory = nil
		w.lastCaptionSnapshot = nil
		w.applyPreviewLines(nil, false)
	})
}

func (w *Window) PushCaption(value string) {
	text := strings.TrimSpace(value)
	if text == "" || w.mainWindow.IsDisposed() {
		return
	}

	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}

		w.pushCaptionLines(splitCaptionLines(text))
		w.applyPreviewLines(w.captionHistory, true)
	})
}

func (w *Window) BringToFront() {
	if w.mainWindow.IsDisposed() {
		return
	}

	w.mainWindow.Synchronize(func() {
		if w.mainWindow.IsDisposed() {
			return
		}
		w.applyTopmostState(true)
	})
}

func (w *Window) applyConfig(config Config) error {
	config = withDefaults(config)
	w.currentConfig = config
	w.alwaysOnTopEnabled = config.AlwaysOnTop
	w.updateActionButtons()
	if err := w.applyPresentation(); err != nil {
		return err
	}

	captionFont, err := walk.NewFont(config.FontFamily, config.FontSize, walk.FontBold)
	if err != nil {
		return err
	}
	w.mainWindow.AddDisposable(captionFont)
	w.previewSurface.SetFont(captionFont)
	w.previewSurface.SetLineColors(
		walk.RGB(config.TextColor.R, config.TextColor.G, config.TextColor.B),
		walk.RGB(config.AlternateTextColor.R, config.AlternateTextColor.G, config.AlternateTextColor.B),
		config.AlternateLineColors,
	)
	w.previewSurface.SetStageColors(
		walk.RGB(maxChannel(config.BackgroundColor.R, 10), maxChannel(config.BackgroundColor.G, 16), maxChannel(config.BackgroundColor.B, 24)),
		walk.RGB(maxChannel(config.BackgroundColor.R/2, 6), maxChannel(config.BackgroundColor.G/2, 10), maxChannel(config.BackgroundColor.B/2, 16)),
	)

	bounds := w.boundsForCurrentState(config)
	w.applyingBounds = true
	defer func() {
		w.applyingBounds = false
	}()
	if err := w.mainWindow.SetBoundsPixels(bounds); err != nil {
		return err
	}
	w.storeBounds(bounds)
	w.applyTopmostState(false)

	return nil
}

func (w *Window) applyPresentation() error {
	if w.focusMode {
		w.mainWindow.SetBackground(w.shellBrush)
		w.shell.SetBackground(w.shellBrush)
		w.previewCard.SetBackground(w.shellBrush)
		if err := setGradientCompositeColor(w.previewPanel, ui.PreviewBackground); err != nil {
			return err
		}
		if err := setGradientCompositeColor(w.previewHeader, ui.PreviewBackground); err != nil {
			return err
		}
		if err := setGradientCompositeColor(w.previewStage, ui.PreviewStageBackground); err != nil {
			return err
		}
		w.headerCard.SetVisible(false)
		w.previewHeader.SetVisible(true)
		w.previewTitle.SetVisible(false)
		w.focusHint.SetVisible(true)
		w.settingsCard.SetVisible(false)
		if err := w.rootLayout.SetSpacing(0); err != nil {
			return err
		}
		if err := w.rootLayout.SetMargins(walk.Margins{}); err != nil {
			return err
		}
		if err := w.previewLayout.SetSpacing(6); err != nil {
			return err
		}
		if err := w.previewLayout.SetMargins(walk.Margins{HNear: 12, VNear: 10, HFar: 12, VFar: 12}); err != nil {
			return err
		}
		if err := w.previewStageLay.SetMargins(walk.Margins{HNear: 22, VNear: 18, HFar: 22, VFar: 18}); err != nil {
			return err
		}
		setWindowAlpha(w.mainWindow.Handle(), normalWindowAlpha)
		return nil
	}

	w.mainWindow.SetBackground(w.shellBrush)
	w.shell.SetBackground(w.shellBrush)
	w.previewCard.SetBackground(w.shellBrush)
	if err := setGradientCompositeColor(w.previewPanel, ui.PreviewBackground); err != nil {
		return err
	}
	if err := setGradientCompositeColor(w.previewHeader, ui.PreviewBackground); err != nil {
		return err
	}
	if err := setGradientCompositeColor(w.previewStage, ui.PreviewStageBackground); err != nil {
		return err
	}
	w.headerCard.SetVisible(true)
	w.previewHeader.SetVisible(true)
	w.previewTitle.SetVisible(true)
	w.focusHint.SetVisible(false)
	w.settingsCard.SetVisible(w.settingsVisible)
	if err := w.rootLayout.SetSpacing(16); err != nil {
		return err
	}
	if err := w.rootLayout.SetMargins(walk.Margins{HNear: 18, VNear: 18, HFar: 18, VFar: 18}); err != nil {
		return err
	}
	if err := w.previewLayout.SetSpacing(10); err != nil {
		return err
	}
	if err := w.previewLayout.SetMargins(walk.Margins{HNear: 16, VNear: 16, HFar: 16, VFar: 16}); err != nil {
		return err
	}
	if err := w.previewStageLay.SetMargins(walk.Margins{HNear: 18, VNear: 18, HFar: 18, VFar: 18}); err != nil {
		return err
	}
	setWindowAlpha(w.mainWindow.Handle(), normalWindowAlpha)
	return nil
}

func (w *Window) updateActionButtons() {
	if w.settingsVisible {
		_ = w.settings.SetText("Hide Settings")
	} else {
		_ = w.settings.SetText("Settings")
	}

	if w.alwaysOnTopEnabled {
		_ = w.alwaysOnTop.SetText("On Top: On")
	} else {
		_ = w.alwaysOnTop.SetText("On Top: Off")
	}
}

func withDefaults(config Config) Config {
	if strings.TrimSpace(config.InitialText) == "" {
		config.InitialText = "Waiting for Live Captions..."
	}
	if strings.TrimSpace(config.FontFamily) == "" {
		config.FontFamily = "Segoe UI"
	}
	if config.FontSize <= 0 {
		config.FontSize = 22
	}
	if config.Height <= 0 {
		config.Height = 96
	}
	if config.BackgroundColor == (Color{}) {
		config.BackgroundColor = Color{R: 24, G: 31, B: 41}
	}
	if config.TextColor == (Color{}) {
		config.TextColor = Color{R: 244, G: 240, B: 233}
	}
	if config.AlternateTextColor == (Color{}) {
		config.AlternateTextColor = Color{R: 255, G: 211, B: 106}
	}
	return config
}

func boundsForState(config Config, settingsVisible bool, focusMode bool) walk.Rectangle {
	screenWidth := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	screenHeight := int(win.GetSystemMetrics(win.SM_CYSCREEN))

	width := 940
	height := 252
	if focusMode {
		width = 1080
		height = config.Height + 124
		if height < 220 {
			height = 220
		}
	} else if settingsVisible {
		width = 1080
		height = 720
	}
	if width > screenWidth-80 {
		width = screenWidth - 80
	}
	if height > screenHeight-80 {
		height = screenHeight - 80
	}
	if width < 760 {
		width = screenWidth - 40
	}
	if height < 220 {
		height = 220
	}

	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return walk.Rectangle{X: x, Y: y, Width: width, Height: height}
}

func (w *Window) boundsForCurrentState(config Config) walk.Rectangle {
	if w.focusMode {
		if isStoredBounds(w.focusBounds) {
			return w.focusBounds
		}
	} else if w.settingsVisible {
		if isStoredBounds(w.expandedBounds) {
			return w.expandedBounds
		}
	} else {
		if isStoredBounds(w.collapsedBounds) {
			return w.collapsedBounds
		}
	}

	return boundsForState(config, w.settingsVisible, w.focusMode)
}

func (w *Window) storeBounds(bounds walk.Rectangle) {
	if !isStoredBounds(bounds) {
		return
	}

	if w.focusMode {
		w.focusBounds = bounds
		return
	}

	if w.settingsVisible {
		w.expandedBounds = bounds
		return
	}

	w.collapsedBounds = bounds
}

func isStoredBounds(bounds walk.Rectangle) bool {
	return bounds.Width > 0 && bounds.Height > 0
}

func (w *Window) applyPreviewLines(lines []previewLine, animate bool) {
	signature := previewLineSignature(lines)
	if signature == w.lastText {
		return
	}

	w.previewSurface.SetLines(lines, animate)
	w.lastText = signature
}

func (w *Window) pushCaptionLines(lines []string) {
	incoming := compactCaptionLines(lines)
	if len(incoming) == 0 {
		return
	}

	if len(w.lastCaptionSnapshot) == 0 {
		w.captionHistory = nil
	}

	if replacement, ok := newestCaptionReplacement(w.lastCaptionSnapshot, incoming); ok {
		w.captionHistory = replaceLastPreviewCaption(w.captionHistory, replacement)
	}

	appended := appendedCaptionLines(w.lastCaptionSnapshot, incoming)
	if len(appended) > 0 {
		w.captionHistory = appendPreviewTextsWithPersistentColors(w.captionHistory, appended...)
	}

	w.captionHistory = trimPreviewHistory(w.captionHistory, w.previewHistoryLimit())
	w.lastCaptionSnapshot = append([]string(nil), incoming...)
}

func appendedCaptionLines(previous []string, incoming []string) []string {
	if len(incoming) == 0 {
		return nil
	}
	if len(previous) == 0 {
		return append([]string(nil), incoming...)
	}

	overlap := findCaptionOverlap(previous, incoming)
	if overlap > 0 {
		return append([]string(nil), incoming[overlap:]...)
	}

	if _, ok := newestCaptionReplacement(previous, incoming); ok {
		return nil
	}

	appended := make([]string, 0, len(incoming))
	for _, line := range incoming {
		if !captionSliceContainsComparable(previous, line) {
			appended = append(appended, line)
		}
	}

	if len(appended) == 0 {
		return nil
	}

	return appended
}

func newestCaptionReplacement(previous []string, incoming []string) (string, bool) {
	if len(previous) == 0 || len(incoming) == 0 {
		return "", false
	}

	previousLast := previous[len(previous)-1]
	incomingLast := incoming[len(incoming)-1]
	if !shouldReplaceCaption(previousLast, incomingLast) && !sameCaptionIdentity(previousLast, incomingLast) {
		return "", false
	}
	if captionComparisonKey(previousLast) == captionComparisonKey(incomingLast) {
		return "", false
	}

	return incomingLast, true
}

func captionSliceContainsComparable(lines []string, target string) bool {
	for _, line := range lines {
		if shouldReplaceCaption(line, target) || sameCaptionIdentity(line, target) {
			return true
		}
	}

	return false
}

func replaceLastPreviewCaption(lines []previewLine, value string) []previewLine {
	if len(lines) == 0 {
		return nil
	}

	updated := append([]previewLine(nil), lines...)
	updated[len(updated)-1].Text = strings.TrimSpace(value)
	return compactPreviewLines(updated)
}

func (w *Window) previewHistoryLimit() int {
	limit := minCaptionHistoryLines
	if w.previewSurface == nil {
		return limit
	}

	widget := w.previewSurface.Widget()
	if widget == nil || widget.IsDisposed() {
		return limit
	}

	height := widget.ClientBoundsPixels().Height
	if height <= 0 {
		return limit
	}

	estimatedLineHeight := w.currentConfig.FontSize + 12
	if estimatedLineHeight < minEstimatedPreviewLineH {
		estimatedLineHeight = minEstimatedPreviewLineH
	}

	perLineHeight := estimatedLineHeight + previewLineGap
	if perLineHeight <= 0 {
		return limit
	}

	visibleLines := height / perLineHeight
	if visibleLines < 1 {
		visibleLines = 1
	}

	limit = visibleLines + historyLineHeadroom
	if limit < minCaptionHistoryLines {
		return minCaptionHistoryLines
	}
	if limit > maxCaptionHistoryLines {
		return maxCaptionHistoryLines
	}

	return limit
}

func mergePreviewHistory(existing []previewLine, incoming []string, limit int) []previewLine {
	incoming = compactCaptionLines(incoming)
	if len(incoming) == 0 {
		return trimPreviewHistory(existing, limit)
	}

	existing = compactPreviewLines(existing)
	if len(existing) == 0 {
		return trimPreviewHistory(appendPreviewTextsWithPersistentColors(nil, incoming...), limit)
	}

	overlap := findCaptionOverlap(previewLineTexts(existing), incoming)
	keepCount := len(existing) - overlap
	merged := append([]previewLine(nil), existing[:keepCount]...)
	for index := 0; index < overlap; index++ {
		preserved := existing[keepCount+index]
		preserved.Text = incoming[index]
		merged = append(merged, preserved)
	}

	merged = appendPreviewTextsWithPersistentColors(merged, incoming[overlap:]...)
	return trimPreviewHistory(merged, limit)
}

func appendPreviewTextsWithPersistentColors(base []previewLine, texts ...string) []previewLine {
	result := append([]previewLine(nil), base...)
	useAlternate := false
	if len(result) > 0 {
		useAlternate = !result[len(result)-1].Alternate
	}

	for _, text := range texts {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}

		assignedAlternate := useAlternate
		if preservedAlternate, ok := findPreviewLineAlternate(result, trimmed); ok {
			assignedAlternate = preservedAlternate
		}

		result = append(result, previewLine{Text: trimmed, Alternate: assignedAlternate})
		useAlternate = !assignedAlternate
	}

	return result
}

func findPreviewLineAlternate(lines []previewLine, text string) (bool, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false, false
	}

	for index := len(lines) - 1; index >= 0; index-- {
		if sameCaptionIdentity(lines[index].Text, trimmed) {
			return lines[index].Alternate, true
		}
	}

	return false, false
}

func sameCaptionIdentity(previous string, next string) bool {
	previousKey := captionComparisonKey(previous)
	nextKey := captionComparisonKey(next)
	if previousKey == "" || nextKey == "" {
		return false
	}

	if previousKey == nextKey {
		return true
	}

	return strings.HasPrefix(previousKey, nextKey) || strings.HasPrefix(nextKey, previousKey)
}

func trimPreviewHistory(lines []previewLine, limit int) []previewLine {
	lines = compactPreviewLines(lines)
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return append([]previewLine(nil), lines...)
}

func compactCaptionLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	compacted := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if len(compacted) > 0 && shouldReplaceCaption(compacted[len(compacted)-1], trimmed) {
			compacted[len(compacted)-1] = trimmed
			continue
		}

		compacted = append(compacted, trimmed)
	}

	return compacted
}

func findCaptionOverlap(existing []string, incoming []string) int {
	maxOverlap := len(existing)
	if len(incoming) < maxOverlap {
		maxOverlap = len(incoming)
	}

	for overlap := maxOverlap; overlap > 0; overlap-- {
		if captionLinesMatch(existing[len(existing)-overlap:], incoming[:overlap]) {
			return overlap
		}
	}

	return 0
}

func captionLinesMatch(existing []string, incoming []string) bool {
	if len(existing) != len(incoming) {
		return false
	}

	for index := range existing {
		if !shouldReplaceCaption(existing[index], incoming[index]) {
			return false
		}
	}

	return true
}

func shouldReplaceCaption(previous string, next string) bool {
	previousKey := captionComparisonKey(previous)
	nextKey := captionComparisonKey(next)
	if previousKey == "" || nextKey == "" {
		return false
	}

	if previousKey == nextKey || strings.HasPrefix(previousKey, nextKey) || strings.HasPrefix(nextKey, previousKey) {
		return true
	}

	previousWords := strings.Fields(previousKey)
	nextWords := strings.Fields(nextKey)
	shorter := len(previousWords)
	if len(nextWords) < shorter {
		shorter = len(nextWords)
	}
	if shorter < 4 {
		return false
	}

	sharedPrefixWords := 0
	for sharedPrefixWords < shorter && previousWords[sharedPrefixWords] == nextWords[sharedPrefixWords] {
		sharedPrefixWords++
	}

	if sharedPrefixWords >= shorter-1 {
		return true
	}

	commonWords, previousUniqueCount, nextUniqueCount := commonComparableWordStats(previousWords, nextWords)
	if commonWords < 4 || previousUniqueCount == 0 || nextUniqueCount == 0 {
		return false
	}

	coveragePrevious := float64(commonWords) / float64(previousUniqueCount)
	coverageNext := float64(commonWords) / float64(nextUniqueCount)
	shorterUniqueCount := previousUniqueCount
	longerUniqueCount := nextUniqueCount
	if shorterUniqueCount > longerUniqueCount {
		shorterUniqueCount, longerUniqueCount = longerUniqueCount, shorterUniqueCount
	}
	lengthRatio := float64(shorterUniqueCount) / float64(longerUniqueCount)

	return coveragePrevious >= 0.6 && coverageNext >= 0.45 && lengthRatio >= 0.45
}

func captionComparisonKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = captionTokenNormalizer.Replace(normalized)
	normalized = strings.Trim(normalized, " .,!?:;-")
	return strings.Join(strings.Fields(normalized), " ")
}

func commonComparableWordStats(previousWords []string, nextWords []string) (int, int, int) {
	previousUnique := make(map[string]struct{}, len(previousWords))
	for _, word := range previousWords {
		compact := compactCaptionToken(word)
		if compact != "" {
			previousUnique[compact] = struct{}{}
		}
	}

	nextUnique := make(map[string]struct{}, len(nextWords))
	for _, word := range nextWords {
		compact := compactCaptionToken(word)
		if compact != "" {
			nextUnique[compact] = struct{}{}
		}
	}

	commonWords := 0
	for word := range previousUnique {
		if _, ok := nextUnique[word]; ok {
			commonWords++
		}
	}

	return commonWords, len(previousUnique), len(nextUnique)
}

func compactCaptionToken(value string) string {
	trimmed := strings.Trim(value, " .,!?:;-\"'()[]{}")
	if trimmed == "" {
		return ""
	}

	runes := []rune(trimmed)
	if len(runes) > 4 {
		return string(runes[:4])
	}

	return trimmed
}

func splitCaptionLines(value string) []string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}

	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = sentenceBreakPattern.ReplaceAllString(normalized, "$1\n")

	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.Join(strings.Fields(strings.TrimSpace(rawLine)), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}

	return compactCaptionLines(lines)
}

func (w *Window) applyTopmostState(bringToFront bool) {
	order := win.HWND_NOTOPMOST
	if w.alwaysOnTopEnabled {
		order = win.HWND_TOPMOST
	} else if bringToFront {
		order = win.HWND_TOP
	}

	flags := uint32(win.SWP_NOMOVE | win.SWP_NOSIZE | win.SWP_SHOWWINDOW)
	if !bringToFront {
		flags |= win.SWP_NOACTIVATE
	}

	_ = win.SetWindowPos(
		w.mainWindow.Handle(),
		order,
		0,
		0,
		0,
		0,
		flags,
	)

	if bringToFront {
		_ = win.SetForegroundWindow(w.mainWindow.Handle())
	}
}

func setWindowAlpha(hwnd win.HWND, alpha byte) {
	style := uint32(win.GetWindowLong(hwnd, win.GWL_EXSTYLE))
	if alpha >= normalWindowAlpha {
		if style&win.WS_EX_LAYERED != 0 {
			win.SetWindowLong(hwnd, win.GWL_EXSTYLE, int32(style&^win.WS_EX_LAYERED))
			_ = win.SetWindowPos(
				hwnd,
				0,
				0,
				0,
				0,
				0,
				win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED,
			)
		}
		refreshWindowTree(hwnd)
		return
	}

	if style&win.WS_EX_LAYERED == 0 {
		win.SetWindowLong(hwnd, win.GWL_EXSTYLE, int32(style|win.WS_EX_LAYERED))
		_ = win.SetWindowPos(
			hwnd,
			0,
			0,
			0,
			0,
			0,
			win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED,
		)
	}
	_, _, _ = setLayeredWindowAttributesProc.Call(
		uintptr(hwnd),
		0,
		uintptr(alpha),
		uintptr(layeredWindowAlphaFlag),
	)
	refreshWindowTree(hwnd)
}

func refreshWindowTree(hwnd win.HWND) {
	if hwnd == 0 {
		return
	}

	win.RedrawWindow(
		hwnd,
		nil,
		0,
		win.RDW_INVALIDATE|win.RDW_ERASE|win.RDW_ALLCHILDREN|win.RDW_FRAME|win.RDW_UPDATENOW,
	)
}

func setGradientCompositeColor(panel *walk.GradientComposite, color walk.Color) error {
	if panel == nil {
		return nil
	}
	if err := panel.SetColor1(color); err != nil {
		return err
	}
	if err := panel.SetColor2(color); err != nil {
		return err
	}
	return nil
}

func applyDarkTitleBar(hwnd win.HWND) {
	enabled := int32(1)
	size := uintptr(unsafe.Sizeof(enabled))
	for _, attribute := range []uintptr{20, 19} {
		result, _, _ := dwmSetWindowAttributeProc.Call(
			uintptr(hwnd),
			attribute,
			uintptr(unsafe.Pointer(&enabled)),
			size,
		)
		if result == 0 {
			return
		}
	}
}

func maxChannel(value byte, fallback byte) byte {
	if value > fallback {
		return value
	}
	return fallback
}
