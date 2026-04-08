//go:build windows

package ui

import (
	"syscall"

	"github.com/lxn/win"
)

const darkModeExplorerTheme = "DarkMode_Explorer"

type nativeThemeWindow interface {
	Handle() win.HWND
}

func ApplyNativeDarkTheme(windows ...nativeThemeWindow) {
	applyWindowThemeByName(darkModeExplorerTheme, windows...)
}

func ApplyNativeDarkThemeToFirstChild(windows ...nativeThemeWindow) {
	theme := utf16PtrOrNil(darkModeExplorerTheme)
	if theme == nil {
		return
	}
	for _, window := range windows {
		if window == nil {
			continue
		}

		child := win.GetWindow(window.Handle(), win.GW_CHILD)
		applyWindowTheme(child, theme)
	}
}

func applyWindowThemeByName(themeName string, windows ...nativeThemeWindow) {
	if themeName == "" {
		return
	}

	theme := utf16PtrOrNil(themeName)
	if theme == nil {
		return
	}
	for _, window := range windows {
		if window == nil {
			continue
		}

		applyWindowTheme(window.Handle(), theme)
	}
}

func applyWindowTheme(hwnd win.HWND, theme *uint16) {
	if hwnd == 0 || theme == nil {
		return
	}

	if hr := win.SetWindowTheme(hwnd, theme, nil); win.FAILED(hr) {
		return
	}

	win.SendMessage(hwnd, win.WM_THEMECHANGED, 0, 0)
	win.InvalidateRect(hwnd, nil, true)
}

func utf16PtrOrNil(value string) *uint16 {
	if value == "" {
		return nil
	}

	pointer, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return nil
	}
	return pointer
}
