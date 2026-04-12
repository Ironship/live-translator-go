//go:build windows

package ui

import (
	"strings"

	"github.com/lxn/walk"
)

// ThemeColors holds all UI palette colors for a single named theme.
type ThemeColors struct {
	AppBackground          walk.Color
	CardBackground         walk.Color
	PanelBackground        walk.Color
	PreviewBackground      walk.Color
	PreviewStageBackground walk.Color
	InputBackground        walk.Color

	TextPrimary   walk.Color
	TextSecondary walk.Color
	TextMuted     walk.Color
	Accent        walk.Color
	AccentSoft    walk.Color
	Error         walk.Color
	Info          walk.Color
	Success       walk.Color
	InputText     walk.Color
}

// ThemeNames lists all available theme names.
var ThemeNames = []string{"Dark", "OLED", "Light", "Amber"}

// ThemeForName returns the ThemeColors for a named theme.
// Unknown names fall back to the Dark theme.
func ThemeForName(name string) ThemeColors {
	switch strings.TrimSpace(name) {
	case "OLED":
		return ThemeColors{
			AppBackground:          walk.RGB(0, 0, 0),
			CardBackground:         walk.RGB(8, 8, 8),
			PanelBackground:        walk.RGB(4, 4, 4),
			PreviewBackground:      walk.RGB(6, 6, 6),
			PreviewStageBackground: walk.RGB(0, 0, 0),
			InputBackground:        walk.RGB(12, 12, 12),
			TextPrimary:            walk.RGB(237, 242, 247),
			TextSecondary:          walk.RGB(167, 179, 193),
			TextMuted:              walk.RGB(116, 128, 142),
			Accent:                 walk.RGB(240, 193, 94),
			AccentSoft:             walk.RGB(196, 151, 92),
			Error:                  walk.RGB(231, 98, 110),
			Info:                   walk.RGB(122, 163, 255),
			Success:                walk.RGB(95, 195, 132),
			InputText:              walk.RGB(244, 246, 250),
		}
	case "Light":
		return ThemeColors{
			AppBackground:          walk.RGB(248, 248, 248),
			CardBackground:         walk.RGB(255, 255, 255),
			PanelBackground:        walk.RGB(240, 241, 245),
			PreviewBackground:      walk.RGB(252, 252, 254),
			PreviewStageBackground: walk.RGB(230, 232, 240),
			InputBackground:        walk.RGB(242, 244, 248),
			TextPrimary:            walk.RGB(15, 20, 30),
			TextSecondary:          walk.RGB(60, 70, 85),
			TextMuted:              walk.RGB(100, 110, 125),
			Accent:                 walk.RGB(160, 100, 10),
			AccentSoft:             walk.RGB(130, 80, 10),
			Error:                  walk.RGB(200, 50, 60),
			Info:                   walk.RGB(50, 100, 210),
			Success:                walk.RGB(40, 150, 80),
			InputText:              walk.RGB(15, 20, 30),
		}
	case "Amber":
		return ThemeColors{
			AppBackground:          walk.RGB(8, 5, 0),
			CardBackground:         walk.RGB(14, 10, 2),
			PanelBackground:        walk.RGB(12, 8, 1),
			PreviewBackground:      walk.RGB(16, 11, 2),
			PreviewStageBackground: walk.RGB(6, 4, 0),
			InputBackground:        walk.RGB(18, 14, 4),
			TextPrimary:            walk.RGB(255, 190, 60),
			TextSecondary:          walk.RGB(200, 145, 40),
			TextMuted:              walk.RGB(150, 105, 25),
			Accent:                 walk.RGB(255, 220, 100),
			AccentSoft:             walk.RGB(220, 170, 60),
			Error:                  walk.RGB(231, 98, 110),
			Info:                   walk.RGB(200, 170, 80),
			Success:                walk.RGB(180, 220, 80),
			InputText:              walk.RGB(255, 190, 60),
		}
	default: // "Dark" and any unknown value
		return ThemeColors{
			AppBackground:          walk.RGB(13, 17, 24),
			CardBackground:         walk.RGB(18, 24, 33),
			PanelBackground:        walk.RGB(16, 21, 29),
			PreviewBackground:      walk.RGB(21, 28, 37),
			PreviewStageBackground: walk.RGB(11, 15, 22),
			InputBackground:        walk.RGB(24, 31, 41),
			TextPrimary:            walk.RGB(237, 242, 247),
			TextSecondary:          walk.RGB(167, 179, 193),
			TextMuted:              walk.RGB(116, 128, 142),
			Accent:                 walk.RGB(240, 193, 94),
			AccentSoft:             walk.RGB(196, 151, 92),
			Error:                  walk.RGB(231, 98, 110),
			Info:                   walk.RGB(122, 163, 255),
			Success:                walk.RGB(95, 195, 132),
			InputText:              walk.RGB(244, 246, 250),
		}
	}
}

var (
	AppBackground          = walk.RGB(13, 17, 24)
	CardBackground         = walk.RGB(18, 24, 33)
	PanelBackground        = walk.RGB(16, 21, 29)
	PreviewBackground      = walk.RGB(21, 28, 37)
	PreviewStageBackground = walk.RGB(11, 15, 22)
	FocusBackground        = walk.RGB(8, 11, 16)
	FocusPanelBackground   = walk.RGB(10, 14, 20)
	FocusStageBackground   = walk.RGB(12, 17, 24)
	InputBackground        = walk.RGB(24, 31, 41)

	TextPrimary   = walk.RGB(237, 242, 247)
	TextSecondary = walk.RGB(167, 179, 193)
	TextMuted     = walk.RGB(116, 128, 142)
	Accent        = walk.RGB(240, 193, 94)
	AccentSoft    = walk.RGB(196, 151, 92)
	Error         = walk.RGB(231, 98, 110)
	Info          = walk.RGB(122, 163, 255)
	Success       = walk.RGB(95, 195, 132)
	InputText     = walk.RGB(244, 246, 250)
)
