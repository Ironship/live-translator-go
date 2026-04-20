//go:build windows

package app

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"live-translator-go/internal/captions"
	"live-translator-go/internal/i18n"
	"live-translator-go/internal/overlay"
	"live-translator-go/internal/settings"
	"live-translator-go/internal/translator"
)

type Controller struct {
	rootCtx context.Context
	overlay *overlay.Window
	panel   *settingsPanel

	mu             sync.Mutex
	values         settings.Values
	pipelineCancel context.CancelFunc
}

const (
	minOverlayFontSize = 12
	maxOverlayFontSize = 64
)

func NewController(rootCtx context.Context, overlayWindow *overlay.Window, values settings.Values) *Controller {
	return &Controller{
		rootCtx: rootCtx,
		overlay: overlayWindow,
		values:  settings.Sanitize(values),
	}
}

func (c *Controller) AttachSettingsPanel(panel *settingsPanel) {
	c.panel = panel
}

func (c *Controller) Start() {
	if !settings.IsConfigured(c.CurrentSettings()) {
		c.overlay.SetStatus("Configuration required before translation can start")
		c.overlay.SetText(translator.MissingConfigurationMessage(c.CurrentSettings().Provider))
		c.ShowSettings()
		return
	}

	c.ApplySettings(c.CurrentSettings())
}

func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipelineCancel != nil {
		c.pipelineCancel()
		c.pipelineCancel = nil
	}
}

func (c *Controller) ToggleSettings() {
	if c.overlay.SettingsVisible() {
		c.HideSettings()
		return
	}
	c.ShowSettings()
}

func (c *Controller) ShowSettings() {
	if c.panel == nil {
		c.overlay.SetText("Settings panel is not available")
		return
	}
	c.panel.Load(c.CurrentSettings())
	c.overlay.SetStatus("Adjust provider, caption source, and appearance settings")
	c.overlay.SetSettingsVisible(true)
}

func (c *Controller) HideSettings() {
	c.overlay.SetSettingsVisible(false)
	if settings.IsConfigured(c.CurrentSettings()) {
		c.overlay.SetStatus("Waiting for Live Captions window. Current provider: " + c.CurrentSettings().Provider)
	} else {
		c.overlay.SetStatus("Configuration required before translation can start")
	}
}

func (c *Controller) ToggleFocusMode() {
	if !c.overlay.FocusModeEnabled() && c.overlay.SettingsVisible() {
		c.overlay.SetSettingsVisible(false)
	}
	_ = c.overlay.ToggleFocusMode()
}

// CycleUILanguage advances the UI locale to the next supported language
// (English -> Polski -> Deutsch -> English), persists it, and asks the
// overlay to refresh its tooltips. The settings panel picks up the new
// value the next time it is opened.
func (c *Controller) CycleUILanguage() {
	current := c.CurrentSettings()
	next := current
	next.UILanguage = i18n.NextLanguage(current.UILanguage)

	if err := settings.Save(next); err != nil {
		c.overlay.SetStatus("Unable to save interface language")
		return
	}

	c.mu.Lock()
	c.values = settings.Sanitize(next)
	c.mu.Unlock()

	c.overlay.SetLanguage(next.UILanguage)
	if c.panel != nil {
		c.panel.Load(c.CurrentSettings())
	}
	c.overlay.SetStatus(i18n.T(next.UILanguage, "toolbar.language") + ": " + i18n.DisplayName(next.UILanguage))
}

func (c *Controller) ToggleWordByWord() {
	current := c.CurrentSettings()
	next := current
	next.WordByWord = !current.WordByWord

	if err := settings.Save(next); err != nil {
		c.overlay.SetStatus("Unable to save word-by-word preference")
		return
	}

	c.mu.Lock()
	c.values = settings.Sanitize(next)
	c.mu.Unlock()

	c.overlay.SetWordByWord(next.WordByWord)

	if next.WordByWord {
		c.overlay.SetStatus("Word-by-word translation enabled")
	} else {
		c.overlay.SetStatus("Word-by-word translation disabled")
	}

	c.ApplySettings(next)
}

func (c *Controller) ToggleAlwaysOnTop() {
	current := c.CurrentSettings()
	next := current
	next.AlwaysOnTop = !current.AlwaysOnTop

	c.overlay.SetAlwaysOnTop(next.AlwaysOnTop)
	if err := settings.Save(next); err != nil {
		c.overlay.SetAlwaysOnTop(current.AlwaysOnTop)
		c.overlay.SetStatus("Unable to save always-on-top preference")
		return
	}

	c.mu.Lock()
	c.values = settings.Sanitize(next)
	c.mu.Unlock()

	if next.AlwaysOnTop {
		c.overlay.SetStatus("Always on top enabled")
		return
	}

	c.overlay.SetStatus("Always on top disabled")
}

func (c *Controller) AdjustFontSize(delta int) {
	if delta == 0 {
		return
	}

	current := c.CurrentSettings()
	next := current
	next.FontSize = clampFontSize(current.FontSize + delta)
	if next.FontSize == current.FontSize {
		c.overlay.SetStatus(fmt.Sprintf("Font size: %d", current.FontSize))
		return
	}

	if err := settings.Save(next); err != nil {
		c.overlay.SetStatus("Unable to save font size preference")
		return
	}

	next = settings.Sanitize(next)
	c.mu.Lock()
	c.values = next
	c.mu.Unlock()

	if c.panel != nil && c.panel.fontSizeRow != nil && c.panel.fontSizeRow.edit != nil && !c.panel.fontSizeRow.edit.IsDisposed() {
		_ = c.panel.fontSizeRow.edit.SetText(strconv.Itoa(next.FontSize))
	}

	if err := c.overlay.ApplyConfig(ConfigFromSettings(next).Overlay); err != nil {
		c.overlay.SetText(fmt.Sprintf("Overlay update error: %v", err))
		return
	}

	c.overlay.SetStatus(fmt.Sprintf("Font size: %d", next.FontSize))
}

func (c *Controller) SaveSettings(updated settings.Values) error {
	updated = settings.Sanitize(updated)
	if err := settings.Save(updated); err != nil {
		return fmt.Errorf("nie udalo sie zapisac ustawien: %w", err)
	}

	c.mu.Lock()
	c.values = updated
	c.mu.Unlock()

	c.overlay.SetSettingsVisible(false)
	c.ApplySettings(updated)
	return nil
}

func (c *Controller) CancelSettings() {
	c.HideSettings()
}

func (c *Controller) ApplySettings(values settings.Values) {
	config := ConfigFromSettings(values)
	c.Stop()
	if err := c.overlay.ApplyConfig(config.Overlay); err != nil {
		c.overlay.SetText(fmt.Sprintf("Overlay update error: %v", err))
		return
	}

	c.overlay.SetWordByWord(values.WordByWord)

	if !settings.IsConfigured(values) {
		c.overlay.SetStatus("Configuration required before translation can start")
		c.overlay.SetText(translator.MissingConfigurationMessage(values.Provider))
		return
	}

	c.overlay.SetStatus("Waiting for Live Captions window. Current provider: " + values.Provider)
	c.overlay.SetText(config.Overlay.InitialText)

	pipelineCtx, cancel := context.WithCancel(c.rootCtx)
	c.mu.Lock()
	c.pipelineCancel = cancel
	c.mu.Unlock()

	go runPipeline(pipelineCtx, config, c.overlay)
}

func (c *Controller) OpenLiveCaptions() {
	mode, err := captions.OpenLiveCaptionsWithRecovery(captions.Config{
		ProcessName:     c.CurrentSettings().CaptionProcessName,
		WindowClassName: c.CurrentSettings().CaptionWindowClass,
		AutomationID:    c.CurrentSettings().CaptionAutomationID,
	})
	if err != nil {
		c.overlay.SetStatus("Unable to open Live Captions automatically")
		c.overlay.SetText(err.Error())
		return
	}

	c.overlay.BringToFront()
	time.AfterFunc(1200*time.Millisecond, c.overlay.BringToFront)

	if !settings.IsConfigured(c.CurrentSettings()) {
		if mode == captions.LaunchModeRestarted {
			c.overlay.SetStatus("Live Captions restart requested. Finish provider setup, then the app will start watching automatically.")
			return
		}
		if mode == captions.LaunchModeDirect {
			c.overlay.SetStatus("Live Captions launch requested. Finish provider setup, then the app will start watching automatically.")
			return
		}
		c.overlay.SetStatus("Accessibility settings opened. Turn on Live Captions there, then finish provider setup.")
		return
	}

	if mode == captions.LaunchModeRestarted {
		c.overlay.SetStatus("Live Captions restart requested. The watcher will attach automatically when the window appears again.")
		return
	}

	if mode == captions.LaunchModeDirect {
		c.overlay.SetStatus("Live Captions launch requested. The watcher will attach automatically when the window appears.")
		return
	}

	c.overlay.SetStatus("Accessibility settings opened. Turn on Live Captions there and the watcher will attach automatically.")
}

func (c *Controller) OpenSpeechRecognitionPanel() {
	if err := captions.OpenSpeechRecognitionPanel(); err != nil {
		c.overlay.SetStatus("Unable to open speech settings")
		c.overlay.SetText(err.Error())
		return
	}

	c.overlay.SetStatus("Speech settings opened. Choose the speech language there and enable the non-native speaker option if needed.")
}

func (c *Controller) CurrentSettings() settings.Values {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.values
}

func clampFontSize(value int) int {
	if value < minOverlayFontSize {
		return minOverlayFontSize
	}
	if value > maxOverlayFontSize {
		return maxOverlayFontSize
	}
	return value
}
