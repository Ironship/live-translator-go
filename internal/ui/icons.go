//go:build windows

package ui

// IconFontFamily is the name of the Windows 10/11 built-in Fluent icon font.
// Glyphs below are Segoe MDL2 Assets private-use-area codepoints and render
// as crisp monochrome icons at any size.
//
// Reference: https://learn.microsoft.com/windows/apps/design/style/segoe-ui-symbol-font
const IconFontFamily = "Segoe MDL2 Assets"

// Fluent glyphs used across the UI. Keep names descriptive so callers pick
// the right one without memorising codepoints.
const (
	IconPlay           = "\uE768" // Play
	IconStop           = "\uE71A" // Stop
	IconMicrophone     = "\uE720" // Microphone
	IconSettings       = "\uE713" // Settings gear
	IconChevronLeft    = "\uE76B" // Back (used for "Hide Settings")
	IconPinned         = "\uE840" // Pinned (always-on-top enabled)
	IconUnpin          = "\uE77A" // Unpin (always-on-top disabled)
	IconWordByWordOn   = "\uE97F" // Character / text effect
	IconWordByWordOff  = "\uE8D2" // Font / Aa
	IconClear          = "\uE894" // Clear
	IconEnterFocus     = "\uE1D9" // Full screen / focus
	IconExitFocus      = "\uE73F" // Back to window / exit focus
	IconClose          = "\uE8BB" // Cancel / close
)
