//go:build windows

package ui

import "github.com/lxn/walk"

var (
	AppBackground          = walk.RGB(11, 13, 15)
	CardBackground         = walk.RGB(20, 24, 26)
	PanelBackground        = walk.RGB(15, 19, 20)
	PreviewBackground      = walk.RGB(18, 23, 25)
	PreviewStageBackground = walk.RGB(7, 9, 12)
	FocusBackground        = walk.RGB(6, 8, 10)
	FocusPanelBackground   = walk.RGB(9, 12, 14)
	FocusStageBackground   = walk.RGB(8, 10, 13)
	InputBackground        = walk.RGB(25, 30, 31)

	TextPrimary   = walk.RGB(246, 248, 244)
	TextSecondary = walk.RGB(213, 222, 218)
	TextMuted     = walk.RGB(156, 171, 166)
	Accent        = walk.RGB(255, 198, 92)
	AccentSoft    = walk.RGB(126, 218, 199)
	Error         = walk.RGB(244, 122, 132)
	Info          = walk.RGB(128, 196, 255)
	Success       = walk.RGB(124, 214, 156)
	InputText     = walk.RGB(247, 248, 244)
)
