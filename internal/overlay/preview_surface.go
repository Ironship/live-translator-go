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
	previewFrameInterval             = 16 * time.Millisecond
	previewLineGap                   = 10
	previewHorizontalPad             = 8
	previewVerticalPad               = 10
	previewMeasureHeight             = 4096
	previewAnimatedIncomingLineLimit = 3
	previewScrollDurationPerLine     = 180 * time.Millisecond
	previewScrollDurationMax         = 420 * time.Millisecond
)

type previewLine struct {
	Text      string
	Alternate bool
}

type previewSurface struct {
	widget              *walk.CustomWidget
	mu                  sync.Mutex
	font                *walk.Font
	textColor           walk.Color
	alternateTextColor  walk.Color
	alternateLineColors bool
	stageTopColor       walk.Color
	stageBottomColor    walk.Color
	lines               []previewLine
	scrollStartOffset   float64
	scrollOffset        float64
	scrollStartedAt     time.Time
	scrollDuration      time.Duration
	scrollAnimating     bool
	stopCh              chan struct{}
	stopped             chan struct{}
}

func newPreviewSurface(parent walk.Container, initial []previewLine, textColor walk.Color, alternateTextColor walk.Color, alternateLineColors bool, stageTopColor walk.Color, stageBottomColor walk.Color) (*previewSurface, error) {
	surface := &previewSurface{
		textColor:           textColor,
		alternateTextColor:  alternateTextColor,
		alternateLineColors: alternateLineColors,
		stageTopColor:       stageTopColor,
		stageBottomColor:    stageBottomColor,
		lines:               append([]previewLine(nil), initial...),
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

func (p *previewSurface) SetLines(lines []previewLine, animate bool) {
	lines = compactPreviewLines(lines)
	p.mu.Lock()
	previous := append([]previewLine(nil), p.lines...)
	p.lines = append([]previewLine(nil), lines...)
	font := p.font
	p.mu.Unlock()

	incomingCount := countIncomingPreviewLines(previous, lines)
	if !animate || !shouldAnimatePreviewTransition(previous, incomingCount) {
		p.mu.Lock()
		p.clearScrollAnimationLocked()
		p.mu.Unlock()
		p.invalidate()
		return
	}

	advance := p.measureScrollAdvance(lines[len(lines)-incomingCount:], font)
	p.mu.Lock()
	if advance <= 0 {
		p.clearScrollAnimationLocked()
	} else {
		p.scrollStartOffset = advance
		p.scrollOffset = advance
		p.scrollStartedAt = time.Now()
		p.scrollDuration = previewScrollDuration(incomingCount)
		p.scrollAnimating = p.scrollDuration > 0
	}
	p.mu.Unlock()

	p.invalidate()
}

func (p *previewSurface) runAnimator() {
	ticker := time.NewTicker(previewFrameInterval)
	defer func() {
		ticker.Stop()
		close(p.stopped)
	}()

	for {
		select {
		case <-p.stopCh:
			return
		case now := <-ticker.C:
			shouldInvalidate := false
			p.mu.Lock()
			if p.scrollAnimating {
				elapsed := now.Sub(p.scrollStartedAt)
				if elapsed >= p.scrollDuration {
					p.clearScrollAnimationLocked()
				} else {
					remaining := 1 - (float64(elapsed) / float64(p.scrollDuration))
					if remaining < 0 {
						remaining = 0
					}
					p.scrollOffset = p.scrollStartOffset * remaining
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
	lines := append([]previewLine(nil), p.lines...)
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
			if err := canvas.DrawTextPixels(layout.Text, font, previewLineColor(textColor, alternateTextColor, layout.Alternate, alternateLineColors), lineBounds, walk.TextCenter|walk.TextWordbreak|walk.TextNoPrefix); err != nil {
				return err
			}
		}

		currentY += layout.Height
	}

	return nil
}

func (p *previewSurface) measureScrollAdvance(lines []previewLine, font *walk.Font) float64 {
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
	Text      string
	Height    int
	Alternate bool
}

func measurePreviewLayouts(canvas *walk.Canvas, font *walk.Font, lines []previewLine, width int) ([]previewLineLayout, int, error) {
	if len(lines) == 0 {
		return nil, 0, nil
	}
	if width <= 0 {
		width = 1
	}

	layouts := make([]previewLineLayout, 0, len(lines))
	totalHeight := 0
	for index, line := range lines {
		text := strings.TrimSpace(line.Text)
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
		layouts = append(layouts, previewLineLayout{Text: text, Height: lineHeight, Alternate: line.Alternate})
	}

	return layouts, totalHeight, nil
}

func countIncomingPreviewLines(previous []previewLine, next []previewLine) int {
	if len(next) == 0 {
		return 0
	}
	if len(previous) == 0 {
		return len(next)
	}

	overlap := previewLineOverlap(previous, next)
	incoming := len(next) - overlap
	if incoming < 0 {
		return 0
	}
	return incoming
}

func shouldAnimatePreviewTransition(previous []previewLine, incomingCount int) bool {
	if len(previous) == 0 {
		return false
	}
	if incomingCount <= 0 {
		return false
	}
	return incomingCount <= previewAnimatedIncomingLineLimit
}

func previewScrollDuration(incomingCount int) time.Duration {
	if incomingCount <= 0 {
		return 0
	}
	duration := time.Duration(incomingCount) * previewScrollDurationPerLine
	if duration > previewScrollDurationMax {
		return previewScrollDurationMax
	}
	return duration
}

func (p *previewSurface) clearScrollAnimationLocked() {
	p.scrollStartOffset = 0
	p.scrollOffset = 0
	p.scrollStartedAt = time.Time{}
	p.scrollDuration = 0
	p.scrollAnimating = false
}

func previewLineColor(primary walk.Color, alternate walk.Color, useAlternate bool, alternateEnabled bool) walk.Color {
	if !alternateEnabled || !useAlternate {
		return primary
	}

	return alternate
}

func compactPreviewLines(lines []previewLine) []previewLine {
	if len(lines) == 0 {
		return nil
	}

	compacted := make([]previewLine, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}

		current := line
		current.Text = text
		if len(compacted) > 0 && shouldReplaceCaption(compacted[len(compacted)-1].Text, current.Text) {
			current.Alternate = compacted[len(compacted)-1].Alternate
			compacted[len(compacted)-1] = current
			continue
		}

		compacted = append(compacted, current)
	}

	return compacted
}

func previewLineTexts(lines []previewLine) []string {
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func previewLineOverlap(previous []previewLine, next []previewLine) int {
	return findCaptionOverlap(previewLineTexts(previous), previewLineTexts(next))
}

func previewLineSignature(lines []previewLine) string {
	var builder strings.Builder
	for _, line := range lines {
		if line.Alternate {
			builder.WriteByte('1')
		} else {
			builder.WriteByte('0')
		}
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(line.Text))
		builder.WriteByte('\n')
	}
	return builder.String()
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
