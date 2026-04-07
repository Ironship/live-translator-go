//go:build windows

package ui

import "github.com/lxn/walk"

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
