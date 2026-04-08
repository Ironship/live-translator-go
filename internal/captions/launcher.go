//go:build windows

package captions

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/zzl/go-win32api/v2/win32"
)

type LaunchMode int

const (
	LaunchModeDirect LaunchMode = iota
	LaunchModeSettings
	LaunchModeRestarted
)

const liveCaptionsShellTarget = "shell:AppsFolder\\{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\\LiveCaptions.exe"

var speechSettingsTargets = []string{
	"ms-settings:speech",
	"ms-settings:privacy-speech",
}

const speechRecognitionControlTarget = "control.exe"

const speechRecognitionControlParameters = "/name Microsoft.SpeechRecognition"

const speechRecognitionAppletTarget = "rundll32.exe"

const speechRecognitionAppletParameters = `shell32.dll,Control_RunDLL "%SystemRoot%\\System32\\Speech\\SpeechUX\\sapi.cpl"`

var liveCaptionsFallbackTargets = []string{
	"ms-settings:accessibility-audio",
	"ms-settings:easeofaccess-closedcaptioning",
}

func OpenLiveCaptionsWithRecovery(config Config) (LaunchMode, error) {
	config = withDefaults(config)
	diagnostics := inspectDiagnostics(config)
	if diagnostics.WindowHung {
		if err := terminateProcessByName(config.ProcessName); err != nil {
			return LaunchModeRestarted, err
		}
		return openLiveCaptionsTargets(LaunchModeRestarted)
	}

	return openLiveCaptionsTargets(LaunchModeDirect)
}

func OpenLiveCaptions() (LaunchMode, error) {
	return OpenLiveCaptionsWithRecovery(withDefaults(Config{}))
}

func openLiveCaptionsTargets(mode LaunchMode) (LaunchMode, error) {
	if err := openShellTarget(liveCaptionsShellTarget); err == nil {
		return mode, nil
	}

	errors := []string{fmt.Sprintf("%s failed", liveCaptionsShellTarget)}
	for _, target := range liveCaptionsFallbackTargets {
		if err := openShellTarget(target); err == nil {
			return LaunchModeSettings, nil
		} else {
			errors = append(errors, fmt.Sprintf("%s failed", target))
		}
	}

	return LaunchModeSettings, fmt.Errorf("unable to open Live Captions or accessibility settings: %s", strings.Join(errors, "; "))
}

func terminateProcessByName(processName string) error {
	if !processRunningByName(processName) {
		return nil
	}

	imageName := strings.TrimSpace(processName)
	if imageName == "" {
		imageName = "LiveCaptions"
	}
	if !strings.HasSuffix(strings.ToLower(imageName), ".exe") {
		imageName += ".exe"
	}

	cmd := exec.Command("taskkill", "/IM", imageName, "/F", "/T")
	if output, err := cmd.CombinedOutput(); err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return fmt.Errorf("unable to terminate %s: %w", imageName, err)
		}
		return fmt.Errorf("unable to terminate %s: %s", imageName, trimmed)
	}

	return nil
}

func OpenSpeechRecognitionPanel() error {
	errors := make([]string, 0, len(speechSettingsTargets)+2)
	for _, target := range speechSettingsTargets {
		if err := openShellTarget(target); err == nil {
			return nil
		} else {
			errors = append(errors, fmt.Sprintf("%s failed", target))
		}
	}

	if err := openCommandTarget(speechRecognitionControlTarget, speechRecognitionControlParameters); err == nil {
		return nil
	} else {
		errors = append(errors, fmt.Sprintf("%s %s failed", speechRecognitionControlTarget, speechRecognitionControlParameters))
	}

	if err := openCommandTarget(speechRecognitionAppletTarget, speechRecognitionAppletParameters); err == nil {
		return nil
	} else {
		errors = append(errors, fmt.Sprintf("%s %s failed", speechRecognitionAppletTarget, speechRecognitionAppletParameters))
	}

	return fmt.Errorf("unable to open speech settings or fallback panels: %s", strings.Join(errors, "; "))
}

func openShellTarget(target string) error {
	return openCommandTarget(target, "")
}

func openCommandTarget(target string, parameters string) error {
	var args *uint16
	if strings.TrimSpace(parameters) != "" {
		args = win32.StrToPwstr(parameters)
	}

	result := win32.ShellExecute(
		0,
		nil,
		win32.StrToPwstr(target),
		args,
		nil,
		win32.SW_SHOWNORMAL,
	)
	if result <= 32 {
		return fmt.Errorf("shell execute returned %d", result)
	}
	return nil
}
