//go:build windows

package captions

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/zzl/go-win32api/v2/win32"
)

var clsidCUIAutomation = syscall.GUID{Data1: 0xFF48DBA4, Data2: 0x60EF, Data3: 0x4201, Data4: [8]byte{0xAA, 0x87, 0x54, 0x10, 0x3E, 0xEF, 0x59, 0x4E}}

type Config struct {
	ProcessName     string
	WindowClassName string
	AutomationID    string
	PollInterval    time.Duration
}

type Event struct {
	Text       string
	CapturedAt time.Time
}

type Diagnostics struct {
	ProcessRunning bool
	WindowFound    bool
	WindowHung     bool
	CaptionFound   bool
}

type Watcher struct {
	config              Config
	availabilityChanged func(bool)
	diagnosticsChanged  func(Diagnostics)
}

type nativeWatcher struct {
	config         Config
	automation     *win32.IUIAutomation
	windowHandle   win32.HWND
	windowElement  *win32.IUIAutomationElement
	captionElement *win32.IUIAutomationElement
	lastText       string
	staleText      string
	stalePolls     int
}

const staleCaptionRebindPolls = 24

func NewWatcher(config Config) *Watcher {
	return &Watcher{config: config}
}

func (w *Watcher) OnAvailabilityChanged(handler func(bool)) {
	w.availabilityChanged = handler
}

func (w *Watcher) OnDiagnosticsChanged(handler func(Diagnostics)) {
	w.diagnosticsChanged = handler
}

func (w *Watcher) Run(ctx context.Context, out chan<- Event) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr := win32.CoInitializeEx(nil, win32.COINIT_APARTMENTTHREADED)
	if win32.FAILED(hr) {
		return fmt.Errorf("initialize COM for caption watcher: %s", win32.HRESULT_ToString(hr))
	}
	defer win32.CoUninitialize()

	automation, err := newAutomationClient()
	if err != nil {
		return err
	}
	defer automation.Release()

	native := &nativeWatcher{
		config:     withDefaults(w.config),
		automation: automation,
	}
	availabilityKnown := false
	lastAvailable := false
	diagnosticsKnown := false
	lastDiagnostics := Diagnostics{}

	ticker := time.NewTicker(native.config.PollInterval)
	defer ticker.Stop()

	for {
		text, diagnostics, err := native.readCaption()
		if err != nil {
			return err
		}

		if !diagnosticsKnown || diagnostics != lastDiagnostics {
			diagnosticsKnown = true
			lastDiagnostics = diagnostics
			if w.diagnosticsChanged != nil {
				w.diagnosticsChanged(diagnostics)
			}
		}

		available := diagnostics.WindowFound && !diagnostics.WindowHung

		if !availabilityKnown || available != lastAvailable {
			availabilityKnown = true
			lastAvailable = available
			if w.availabilityChanged != nil {
				w.availabilityChanged(available)
			}
		}

		if text != "" && text != native.lastText {
			native.lastText = text
			event := Event{Text: text, CapturedAt: time.Now()}
			select {
			case out <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func withDefaults(config Config) Config {
	if strings.TrimSpace(config.ProcessName) == "" {
		config.ProcessName = "LiveCaptions"
	}
	if strings.TrimSpace(config.WindowClassName) == "" {
		config.WindowClassName = "LiveCaptionsDesktopWindow"
	}
	if strings.TrimSpace(config.AutomationID) == "" {
		config.AutomationID = "CaptionsTextBlock"
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 350 * time.Millisecond
	}
	return config
}

func newAutomationClient() (*win32.IUIAutomation, error) {
	var automation *win32.IUIAutomation
	hr := win32.CoCreateInstance(
		&clsidCUIAutomation,
		nil,
		win32.CLSCTX_INPROC_SERVER,
		&win32.IID_IUIAutomation,
		unsafe.Pointer(&automation),
	)
	if win32.FAILED(hr) {
		return nil, fmt.Errorf("create UI Automation client: %s", win32.HRESULT_ToString(hr))
	}
	if automation == nil {
		return nil, fmt.Errorf("create UI Automation client: nil automation instance")
	}
	return automation, nil
}

func (w *nativeWatcher) readCaption() (string, Diagnostics, error) {
	diagnostics := Diagnostics{}
	hwnd := findWindow(w.config.WindowClassName, w.config.ProcessName)
	if hwnd == 0 {
		diagnostics.ProcessRunning = processRunningByName(w.config.ProcessName)
		w.releaseBindings()
		w.lastText = ""
		return "", diagnostics, nil
	}
	diagnostics.ProcessRunning = true
	diagnostics.WindowFound = true
	if win32.IsHungAppWindow(hwnd) != 0 {
		w.releaseBindings()
		diagnostics.WindowHung = true
		return "", diagnostics, nil
	}

	if hwnd != w.windowHandle || w.windowElement == nil {
		w.releaseBindings()
		w.windowHandle = hwnd

		var windowElement *win32.IUIAutomationElement
		hr := w.automation.ElementFromHandle(hwnd, &windowElement)
		if win32.FAILED(hr) {
			return "", diagnostics, fmt.Errorf("bind Live Captions window: %s", win32.HRESULT_ToString(hr))
		}
		w.windowElement = windowElement
	}

	if w.captionElement == nil {
		condition, err := createStringCondition(w.automation, win32.UIA_AutomationIdPropertyId, w.config.AutomationID)
		if err != nil {
			return "", diagnostics, err
		}
		defer condition.Release()

		var captionElement *win32.IUIAutomationElement
		hr := w.windowElement.FindFirst(win32.TreeScope_Descendants, condition, &captionElement)
		if win32.FAILED(hr) {
			w.releaseCaptionElement()
			return "", diagnostics, nil
		}
		if captionElement == nil {
			return "", diagnostics, nil
		}
		w.captionElement = captionElement
	}
	diagnostics.CaptionFound = w.captionElement != nil

	text, ok := currentElementName(w.captionElement)
	if !ok {
		w.releaseCaptionElement()
		return "", diagnostics, nil
	}

	trimmed := strings.TrimSpace(text)
	w.trackStaleness(trimmed)
	return trimmed, diagnostics, nil
}

func (w *nativeWatcher) releaseBindings() {
	w.releaseCaptionElement()
	if w.windowElement != nil {
		w.windowElement.Release()
		w.windowElement = nil
	}
	w.windowHandle = 0
	w.staleText = ""
	w.stalePolls = 0
}

func (w *nativeWatcher) releaseCaptionElement() {
	if w.captionElement != nil {
		w.captionElement.Release()
		w.captionElement = nil
	}
}

func (w *nativeWatcher) trackStaleness(text string) {
	if text == "" {
		w.staleText = ""
		w.stalePolls = 0
		return
	}

	if text != w.staleText {
		w.staleText = text
		w.stalePolls = 0
		return
	}

	w.stalePolls++
	if w.stalePolls < staleCaptionRebindPolls {
		return
	}

	w.releaseBindings()
	w.stalePolls = 0
}

func currentElementName(element *win32.IUIAutomationElement) (string, bool) {
	if element == nil {
		return "", false
	}

	var value win32.BSTR
	hr := element.Get_CurrentName(&value)
	if win32.FAILED(hr) {
		return "", false
	}

	return win32.BstrToStrAndFree(value), true
}

func createStringCondition(automation *win32.IUIAutomation, propertyID win32.UIA_PROPERTY_ID, value string) (*win32.IUIAutomationCondition, error) {
	variant := win32.VARIANT{}
	variant.Vt = win32.VT_BSTR
	*variant.BstrVal() = win32.StrToBstr(value)
	defer win32.VariantClear(&variant)

	var condition *win32.IUIAutomationCondition
	hr := automation.CreatePropertyCondition(propertyID, variant, &condition)
	if win32.FAILED(hr) {
		return nil, fmt.Errorf("create UI Automation condition: %s", win32.HRESULT_ToString(hr))
	}
	if condition == nil {
		return nil, fmt.Errorf("create UI Automation condition: nil condition")
	}
	return condition, nil
}

func findWindow(className string, processName string) win32.HWND {
	hwnd, _ := win32.FindWindow(win32.StrToPwstr(className), nil)
	if hwnd == 0 {
		return 0
	}
	if processName == "" || windowMatchesProcessName(hwnd, processName) {
		return hwnd
	}
	return 0
}

func windowMatchesProcessName(hwnd win32.HWND, expected string) bool {
	var processID uint32
	win32.GetWindowThreadProcessId(hwnd, &processID)
	if processID == 0 {
		return false
	}

	actual, err := processNameByID(processID)
	if err != nil {
		return false
	}

	return normalizeProcessName(actual) == normalizeProcessName(expected)
}

func processNameByID(processID uint32) (string, error) {
	snapshot, _ := win32.CreateToolhelp32Snapshot(win32.TH32CS_SNAPPROCESS, 0)
	if snapshot == win32.INVALID_HANDLE_VALUE {
		return "", fmt.Errorf("create process snapshot failed")
	}
	defer win32.CloseHandle(snapshot)

	entry := win32.PROCESSENTRY32W{DwSize: uint32(unsafe.Sizeof(win32.PROCESSENTRY32W{}))}
	ok, _ := win32.Process32FirstW(snapshot, &entry)
	for ok != 0 {
		if entry.Th32ProcessID == processID {
			return win32.WstrToStr(entry.SzExeFile[:]), nil
		}
		ok, _ = win32.Process32NextW(snapshot, &entry)
	}

	return "", fmt.Errorf("process %d not found", processID)
}

func normalizeProcessName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(filepath.Base(value)))
	return strings.TrimSuffix(trimmed, ".exe")
}

func inspectDiagnostics(config Config) Diagnostics {
	config = withDefaults(config)
	hwnd := findWindow(config.WindowClassName, config.ProcessName)
	if hwnd == 0 {
		return Diagnostics{ProcessRunning: processRunningByName(config.ProcessName)}
	}

	return Diagnostics{
		ProcessRunning: true,
		WindowFound:    true,
		WindowHung:     win32.IsHungAppWindow(hwnd) != 0,
	}
}

func processRunningByName(expected string) bool {
	normalizedExpected := normalizeProcessName(expected)
	if normalizedExpected == "" {
		return false
	}

	snapshot, _ := win32.CreateToolhelp32Snapshot(win32.TH32CS_SNAPPROCESS, 0)
	if snapshot == win32.INVALID_HANDLE_VALUE {
		return false
	}
	defer win32.CloseHandle(snapshot)

	entry := win32.PROCESSENTRY32W{DwSize: uint32(unsafe.Sizeof(win32.PROCESSENTRY32W{}))}
	ok, _ := win32.Process32FirstW(snapshot, &entry)
	for ok != 0 {
		if normalizeProcessName(win32.WstrToStr(entry.SzExeFile[:])) == normalizedExpected {
			return true
		}
		ok, _ = win32.Process32NextW(snapshot, &entry)
	}

	return false
}
