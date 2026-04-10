//go:build windows

package app

import (
	"context"
	"errors"
	"fmt"

	"live-translator-go/internal/captions"
	"live-translator-go/internal/overlay"
	"live-translator-go/internal/pipeline"
)

func Run() error {
	values, err := LoadSettings()
	if err != nil {
		return err
	}

	config := ConfigFromSettings(values)

	window, err := overlay.New(config.Overlay)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	controller := NewController(ctx, window, values)
	panel, err := newSettingsPanel(window.SettingsHost(), values, controller.SaveSettings, controller.CancelSettings)
	if err != nil {
		_ = window.Close()
		cancel()
		return err
	}
	controller.AttachSettingsPanel(panel)
	window.OnDisposing(func() {
		controller.Stop()
		cancel()
	})
	window.OnSettings(func() {
		controller.ToggleSettings()
	})
	window.OnOpenLiveCaptions(func() {
		controller.OpenLiveCaptions()
	})
	window.OnOpenSpeechRecognition(func() {
		controller.OpenSpeechRecognitionPanel()
	})
	window.OnToggleAlwaysOnTop(func() {
		controller.ToggleAlwaysOnTop()
	})
	window.OnIncreaseFontSize(func() {
		controller.AdjustFontSize(1)
	})
	window.OnDecreaseFontSize(func() {
		controller.AdjustFontSize(-1)
	})
	window.OnToggleFocusMode(func() {
		controller.ToggleFocusMode()
	})
	window.OnClear(func() {
		window.SetText("")
	})
	window.OnExit(func() {
		_ = window.Close()
	})
	window.OnStarted(func() {
		controller.Start()
	})

	window.Run()
	controller.Stop()
	cancel()
	return nil
}

func runPipeline(ctx context.Context, config Config, output *overlay.Window) {
	watcher := captions.NewWatcher(config.Captions)
	watcher.OnDiagnosticsChanged(func(diagnostics captions.Diagnostics) {
		switch {
		case diagnostics.WindowHung:
			output.SetStatus(fmt.Sprintf("Live Captions process is running but not responding. Current provider: %s. Click Start to restart it.", config.Translator.Provider))
		case diagnostics.WindowFound:
			output.BringToFront()
			output.SetStatus(fmt.Sprintf("Watching Live Captions window. Current provider: %s", config.Translator.Provider))
		case diagnostics.ProcessRunning:
			output.SetStatus(fmt.Sprintf("Live Captions process is running, but its window is not available yet. Current provider: %s", config.Translator.Provider))
		default:
			output.SetStatus(fmt.Sprintf("Waiting for Live Captions window. Current provider: %s. Click Start if it is not running.", config.Translator.Provider))
		}
	})
	translatorClient := config.Translator.NewClient()
	processor := pipeline.NewProcessor(
		pipeline.Config{RequestTimeout: config.RequestTimeout, IdleFlushDelay: config.RequestFrequency},
		translatorClient,
		output,
	)
	defer processor.Close()

	events := make(chan captions.Event, 16)
	errorsCh := make(chan error, 1)

	go func() {
		errorsCh <- watcher.Run(ctx, events)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errorsCh:
			if err == nil || errors.Is(err, context.Canceled) {
				return
			}

			output.SetStatus("Live Captions watcher stopped because of an error")
			output.SetText(fmt.Sprintf("Live Captions watcher error: %v", err))
			return
		case event := <-events:
			processor.Submit(ctx, event.Text)
		}
	}
}
