//go:build windows

package overlay

import (
	"math"
	"strings"
	"sync"
	"time"

	"github.com/lxn/walk"
)

const (
	previewFrameInterval = 16 * time.Millisecond
	previewScrollSpeed   = 220.0
	previewLineGap       = 10
	previewHorizontalPad = 8
	previewVerticalPad   = 10
	previewMeasureHeight = 4096
)

type previewSurface struct {
	widget              *walk.CustomWidget
	mu                  sync.Mutex
	font                *walk.Font
	textColor           walk.Color
	alternateTextColor  walk.Color
	alternateLineColors bool
	stageTopColor       walk.Color
	stageBottomColor    walk.Color
	lines               []string
	scrollOffset        float64
	stopCh              chan struct{}
	stopped             chan struct{}
}

func newPreviewSurface(parent walk.Container, initial []string, textColor walk.Color, alternateTextColor walk.Color, alternateLineColors bool, stageTopColor walk.Color, stageBottomColor walk.Color) (*previewSurface, error) {
	surface := &previewSurface{
		textColor:           textColor,
		alternateTextColor:  alternateTextColor,
		alternateLineColors: alternateLineColors,
		stageTopColor:       stageTopColor,
		stageBottomColor:    stageBottomColor,
		lines:               append([]string(nil), initial...),
		stopCh:              make(chan struct{}),
		stopped:             make(chan struct{}),
	}

	widget, err := walk.NewCustomWidgetPixels(parent, 0, surface.paint)
	if err != nil {
		return nil, err
	}
	widget.SetPaintMode(walk.PaintBuffered)
	widget.SetInvalidatesOnResize(true)
	surface.widget = widget

	go surface.runAnimator()
	return surface, nil
}

func (p *previewSurface) Widget() *walk.CustomWidget {
	return p.widget
}

func (p *previewSurface) Stop() {
	select {
	case <-p.stopCh:
		return
	default:
		close(p.stopCh)
	}
}

func (p *previewSurface) SetFont(font *walk.Font) {
	p.mu.Lock()
	p.font = font
	p.mu.Unlock()
	p.invalidate()
}

func (p *previewSurface) SetLineColors(color walk.Color, alternateColor walk.Color, alternateEnabled bool) {
	p.mu.Lock()
	p.textColor = color
	p.alternateTextColor = alternateColor
	p.alternateLineColors = alternateEnabled
	p.mu.Unlock()
	p.invalidate()
}

func (p *previewSurface) SetStageColors(top walk.Color, bottom walk.Color) {
	p.mu.Lock()
	p.stageTopColor = top
	p.stageBottomColor = bottom
	p.mu.Unlock()
	p.invalidate()
}

func (p *previewSurface) SetLines(lines []string, animate bool) {
	lines = compactCaptionLines(lines)
	p.mu.Lock()
	previous := append([]string(nil), p.lines...)
	p.lines = append([]string(nil), lines...)
	font := p.font
	p.mu.Unlock()

	if animate {
		incomingCount := countIncomingPreviewLines(previous, lines)
		if incomingCount > 0 {
			advance := p.measureScrollAdvance(lines[len(lines)-incomingCount:], font)
			p.mu.Lock()
			p.scrollOffset += advance
			p.mu.Unlock()
		}
	}

	p.invalidate()
}

func (p *previewSurface) runAnimator() {
	ticker := time.NewTicker(previewFrameInterval)
	defer func() {
		ticker.Stop()
		close(p.stopped)
	}()

	lastTick := time.Now()
	for {
		select {
		case <-p.stopCh:
			return
		case now := <-ticker.C:
			delta := now.Sub(lastTick).Seconds()
			lastTick = now

			shouldInvalidate := false
			p.mu.Lock()
			if p.scrollOffset > 0 {
				p.scrollOffset -= previewScrollSpeed * delta
				if p.scrollOffset < 0 {
					p.scrollOffset = 0
				}
				shouldInvalidate = true
			}
			p.mu.Unlock()

			if shouldInvalidate {
				p.invalidate()
			}
		}
	}
}

func (p *previewSurface) invalidate() {
	if p.widget == nil || p.widget.IsDisposed() {
		return
	}

	p.widget.Synchronize(func() {
		if p.widget == nil || p.widget.IsDisposed() {
			return
		}
		_ = p.widget.Invalidate()
	})
}

func (p *previewSurface) paint(canvas *walk.Canvas, updateBounds walk.Rectangle) error {
	bounds := p.widget.ClientBoundsPixels()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		bounds = updateBounds
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return nil
	}

	p.mu.Lock()
	lines := append([]string(nil), p.lines...)
	font := p.font
	textColor := p.textColor
	alternateTextColor := p.alternateTextColor
	alternateLineColors := p.alternateLineColors
	stageTopColor := p.stageTopColor
	stageBottomColor := p.stageBottomColor
	scrollOffset := p.scrollOffset
	p.mu.Unlock()

	if err := canvas.GradientFillRectanglePixels(stageTopColor, stageBottomColor, walk.Vertical, bounds); err != nil {
		return err
	}

	if font == nil {
		return nil
	}

	textBounds := walk.Rectangle{
		X:      previewHorizontalPad,
		Y:      previewVerticalPad,
		Width:  maxInt(bounds.Width-(previewHorizontalPad*2), 1),
		Height: maxInt(bounds.Height-(previewVerticalPad*2), 1),
	}
	layouts, totalHeight, err := measurePreviewLayouts(canvas, font, lines, textBounds.Width)
	if err != nil {
		return err
	}

	startY := bounds.Height - previewVerticalPad - totalHeight + int(math.Round(scrollOffset))
	currentY := startY
	for index, layout := range layouts {
		if index > 0 {
			currentY += previewLineGap
		}

		lineBounds := walk.Rectangle{
			X:      textBounds.X,
			Y:      currentY,
			Width:  textBounds.Width,
			Height: layout.Height,
		}
		if lineBounds.Y+lineBounds.Height >= 0 && lineBounds.Y <= bounds.Height {
			shadowBounds := lineBounds
			shadowBounds.X += 2
			shadowBounds.Y += 2
			if err := canvas.DrawTextPixels(layout.Text, font, walk.RGB(8, 10, 15), shadowBounds, walk.TextCenter|walk.TextWordbreak|walk.TextNoPrefix); err != nil {
				return err
			}
			if err := canvas.DrawTextPixels(layout.Text, font, previewLineColor(textColor, alternateTextColor, index, len(layouts), alternateLineColors), lineBounds, walk.TextCenter|walk.TextWordbreak|walk.TextNoPrefix); err != nil {
				return err
			}
		}

		currentY += layout.Height
	}

	return nil
}

func (p *previewSurface) measureScrollAdvance(lines []string, font *walk.Font) float64 {
	if len(lines) == 0 {
		return 0
	}
	if font == nil || p.widget == nil || p.widget.IsDisposed() {
		return float64(len(lines) * (previewLineGap + 42))
	}

	width := p.widget.ClientBoundsPixels().Width - (previewHorizontalPad * 2)
	if width <= 0 {
		return float64(len(lines) * (previewLineGap + 42))
	}

	bmp, err := walk.NewBitmapForDPI(walk.Size{Width: maxInt(width, 1), Height: 1}, p.widget.DPI())
	if err != nil {
		return float64(len(lines) * (previewLineGap + 42))
	}
	defer bmp.Dispose()

	canvas, err := walk.NewCanvasFromImage(bmp)
	if err != nil {
		return float64(len(lines) * (previewLineGap + 42))
	}
	defer canvas.Dispose()

	layouts, _, err := measurePreviewLayouts(canvas, font, lines, width)
	if err != nil {
		return float64(len(lines) * (previewLineGap + 42))
	}

	advance := 0
	for _, layout := range layouts {
		advance += layout.Height + previewLineGap
	}
	return float64(advance)
}

type previewLineLayout struct {
	Text   string
	Height int
}

func measurePreviewLayouts(canvas *walk.Canvas, font *walk.Font, lines []string, width int) ([]previewLineLayout, int, error) {
	if len(lines) == 0 {
		return nil, 0, nil
	}
	if width <= 0 {
		width = 1
	}

	layouts := make([]previewLineLayout, 0, len(lines))
	totalHeight := 0
	for index, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}

		measured, _, err := canvas.MeasureTextPixels(text, font, walk.Rectangle{Width: width, Height: previewMeasureHeight}, walk.TextCenter|walk.TextWordbreak|walk.TextNoPrefix)
		if err != nil {
			return nil, 0, err
		}
		lineHeight := measured.Height
		if lineHeight <= 0 {
			lineHeight = 32
		}

		if index > 0 {
			totalHeight += previewLineGap
		}
		totalHeight += lineHeight
		layouts = append(layouts, previewLineLayout{Text: text, Height: lineHeight})
	}

	return layouts, totalHeight, nil
}

func countIncomingPreviewLines(previous []string, next []string) int {
	if len(next) == 0 {
		return 0
	}
	if len(previous) == 0 {
		return len(next)
	}

	overlap := findCaptionOverlap(previous, next)
	incoming := len(next) - overlap
	if incoming < 0 {
		return 0
	}
	return incoming
}

func previewLineColor(primary walk.Color, alternate walk.Color, index int, total int, alternateEnabled bool) walk.Color {
	if !alternateEnabled || total <= 1 {
		return primary
	}

	if (total-1-index)%2 == 0 {
		return primary
	}

	return alternate
}

func blendPreviewColor(primary walk.Color, secondary walk.Color, primaryWeight float64) walk.Color {
	if primaryWeight < 0 {
		primaryWeight = 0
	}
	if primaryWeight > 1 {
		primaryWeight = 1
	}
	secondaryWeight := 1 - primaryWeight

	red := int(math.Round((float64(primary&0xFF) * primaryWeight) + (float64(secondary&0xFF) * secondaryWeight)))
	green := int(math.Round((float64((primary>>8)&0xFF) * primaryWeight) + (float64((secondary>>8)&0xFF) * secondaryWeight)))
	blue := int(math.Round((float64((primary>>16)&0xFF) * primaryWeight) + (float64((secondary>>16)&0xFF) * secondaryWeight)))
	return walk.RGB(byte(red), byte(green), byte(blue))
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
