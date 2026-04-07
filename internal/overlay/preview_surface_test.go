//go:build windows

package overlay

import (
	"testing"

	"github.com/lxn/walk"
)

func TestCountIncomingPreviewLines(t *testing.T) {
	incoming := countIncomingPreviewLines(
		[]previewLine{{Text: "First line."}, {Text: "Second line."}},
		[]previewLine{{Text: "First line."}, {Text: "Second line."}, {Text: "Third line."}},
	)
	if incoming != 1 {
		t.Fatalf("expected 1 incoming line, got %d", incoming)
	}
}

func TestCountIncomingPreviewLinesAfterRotation(t *testing.T) {
	incoming := countIncomingPreviewLines(
		[]previewLine{{Text: "One."}, {Text: "Two."}, {Text: "Three."}, {Text: "Four."}, {Text: "Five."}},
		[]previewLine{{Text: "Two."}, {Text: "Three."}, {Text: "Four."}, {Text: "Five."}, {Text: "Six."}},
	)
	if incoming != 1 {
		t.Fatalf("expected 1 incoming line after rotation, got %d", incoming)
	}
}

func TestCountIncomingPreviewLinesIgnoresCorrection(t *testing.T) {
	incoming := countIncomingPreviewLines(
		[]previewLine{{Text: "I think we should do this."}},
		[]previewLine{{Text: "I think we should do this now."}},
	)
	if incoming != 0 {
		t.Fatalf("expected 0 incoming lines for a correction, got %d", incoming)
	}
}

func TestPreviewLineColorUsesPrimaryWhenAlternatingDisabled(t *testing.T) {
	primary := walk.RGB(10, 20, 30)
	alternate := walk.RGB(200, 210, 220)

	if color := previewLineColor(primary, alternate, true, false); color != primary {
		t.Fatalf("expected primary color when alternating is disabled, got %#x", uint32(color))
	}
}

func TestPreviewLineColorUsesAssignedAlternateColor(t *testing.T) {
	primary := walk.RGB(10, 20, 30)
	alternate := walk.RGB(200, 210, 220)

	if color := previewLineColor(primary, alternate, false, true); color != primary {
		t.Fatalf("expected primary color for non-alternate line, got %#x", uint32(color))
	}
	if color := previewLineColor(primary, alternate, true, true); color != alternate {
		t.Fatalf("expected assigned alternate color, got %#x", uint32(color))
	}
}

func TestMergePreviewHistoryKeepsLineAssignmentsDuringScroll(t *testing.T) {
	history := mergePreviewHistory(nil, []string{"One.", "Two.", "Three.", "Four.", "Five."}, 24)
	history = mergePreviewHistory(history, []string{"Six."}, 5)

	expectedTexts := []string{"Two.", "Three.", "Four.", "Five.", "Six."}
	expectedAlternate := []bool{true, false, true, false, true}
	if len(history) != len(expectedTexts) {
		t.Fatalf("expected %d lines, got %d", len(expectedTexts), len(history))
	}

	for index, line := range history {
		if line.Text != expectedTexts[index] {
			t.Fatalf("unexpected text at %d: %q", index, line.Text)
		}
		if line.Alternate != expectedAlternate[index] {
			t.Fatalf("unexpected color assignment at %d: got %t want %t", index, line.Alternate, expectedAlternate[index])
		}
	}
}

func TestCompactPreviewLinesKeepsColorOnCorrection(t *testing.T) {
	lines := compactPreviewLines([]previewLine{
		{Text: "I think we should do this.", Alternate: true},
		{Text: "I think we should do this now.", Alternate: false},
	})

	if len(lines) != 1 {
		t.Fatalf("expected one corrected line, got %d", len(lines))
	}
	if lines[0].Text != "I think we should do this now." {
		t.Fatalf("unexpected corrected text: %q", lines[0].Text)
	}
	if !lines[0].Alternate {
		t.Fatalf("expected corrected line to keep original color assignment")
	}
}

func TestAppendPreviewTextsWithPersistentColorsReusesMatchingAssignment(t *testing.T) {
	base := []previewLine{
		{Text: "This is the first line.", Alternate: false},
		{Text: "This is the second line.", Alternate: true},
	}

	lines := appendPreviewTextsWithPersistentColors(base, "This is the first line now.", "A brand new line.")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	if lines[2].Alternate != base[0].Alternate {
		t.Fatalf("expected matching line to reuse previous assignment")
	}
	if lines[3].Alternate == lines[2].Alternate {
		t.Fatalf("expected next new line to flip assignment after reused color")
	}
}

func TestMergePreviewHistoryPreservesAssignmentWhenOverlapMissing(t *testing.T) {
	history := mergePreviewHistory(nil, []string{"Alpha one.", "Beta two.", "Gamma three."}, 8)
	history = mergePreviewHistory(history, []string{"Alpha one.", "Delta four."}, 8)

	if len(history) < 5 {
		t.Fatalf("expected merged history to include appended lines, got %d", len(history))
	}

	if history[3].Alternate != history[0].Alternate {
		t.Fatalf("expected repeated line to keep original assignment")
	}
}

func TestShouldAnimatePreviewTransition(t *testing.T) {
	if shouldAnimatePreviewTransition(nil, 1) {
		t.Fatalf("expected no animation without previous lines")
	}
	if shouldAnimatePreviewTransition([]previewLine{{Text: "One."}}, 0) {
		t.Fatalf("expected no animation without incoming lines")
	}
	if shouldAnimatePreviewTransition([]previewLine{{Text: "One."}}, previewAnimatedIncomingLineLimit+1) {
		t.Fatalf("expected no animation above incoming line limit")
	}
	if !shouldAnimatePreviewTransition([]previewLine{{Text: "One."}}, 1) {
		t.Fatalf("expected animation for a normal single-line transition")
	}
}

func TestPreviewScrollDurationIsCapped(t *testing.T) {
	if duration := previewScrollDuration(1); duration != previewScrollDurationPerLine {
		t.Fatalf("unexpected single-line duration: %s", duration)
	}
	if duration := previewScrollDuration(10); duration != previewScrollDurationMax {
		t.Fatalf("expected capped duration, got %s", duration)
	}
}

func TestAppendedCaptionLinesAddsOnlyNewTailFromNextSnapshot(t *testing.T) {
	previous := []string{
		"Co?",
		"Ktoś wspiera Kickstarter i wtedy z kolei otrzymuje nagrodę.",
		"Jaka nagroda?",
	}
	incoming := []string{
		"Jaka nagroda?",
		"Nie, nie wiem.",
	}

	appended := appendedCaptionLines(previous, incoming)
	if len(appended) != 1 {
		t.Fatalf("expected one appended line, got %d (%#v)", len(appended), appended)
	}
	if appended[0] != "Nie, nie wiem." {
		t.Fatalf("unexpected appended line: %q", appended[0])
	}
}

func TestAppendedCaptionLinesAvoidsAppendingRepeatedBlock(t *testing.T) {
	previous := []string{
		"Bylibyśmy.",
		"Co?",
		"Ktoś wspiera Kickstarter i wtedy z kolei otrzymuje nagrodę.",
		"Jaka nagroda?",
		"Nie, nie wiem.",
		"Wydobędziesz złoto, a wtedy nie będziemy już rabusiami.",
	}
	incoming := []string{
		"Co?",
		"Ktoś wspiera Kickstarter i wtedy z kolei otrzymuje nagrodę.",
		"Jaka nagroda?",
		"Nie, nie wiem.",
		"Koszulka czy coś",
	}

	appended := appendedCaptionLines(previous, incoming)
	if len(appended) != 1 {
		t.Fatalf("expected only one truly new line, got %d (%#v)", len(appended), appended)
	}
	if appended[0] != "Koszulka czy coś" {
		t.Fatalf("unexpected appended line: %q", appended[0])
	}
}
