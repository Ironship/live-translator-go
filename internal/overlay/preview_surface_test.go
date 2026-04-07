//go:build windows

package overlay

import (
	"testing"

	"github.com/lxn/walk"
)

func TestCountIncomingPreviewLines(t *testing.T) {
	incoming := countIncomingPreviewLines(
		[]string{"First line.", "Second line."},
		[]string{"First line.", "Second line.", "Third line."},
	)
	if incoming != 1 {
		t.Fatalf("expected 1 incoming line, got %d", incoming)
	}
}

func TestCountIncomingPreviewLinesAfterRotation(t *testing.T) {
	incoming := countIncomingPreviewLines(
		[]string{"One.", "Two.", "Three.", "Four.", "Five."},
		[]string{"Two.", "Three.", "Four.", "Five.", "Six."},
	)
	if incoming != 1 {
		t.Fatalf("expected 1 incoming line after rotation, got %d", incoming)
	}
}

func TestCountIncomingPreviewLinesIgnoresCorrection(t *testing.T) {
	incoming := countIncomingPreviewLines(
		[]string{"I think we should do this."},
		[]string{"I think we should do this now."},
	)
	if incoming != 0 {
		t.Fatalf("expected 0 incoming lines for a correction, got %d", incoming)
	}
}

func TestPreviewLineColorUsesPrimaryWhenAlternatingDisabled(t *testing.T) {
	primary := walk.RGB(10, 20, 30)
	alternate := walk.RGB(200, 210, 220)

	if color := previewLineColor(primary, alternate, 1, 4, false); color != primary {
		t.Fatalf("expected primary color when alternating is disabled, got %#x", uint32(color))
	}
}

func TestPreviewLineColorAlternatesFromNewestLine(t *testing.T) {
	primary := walk.RGB(10, 20, 30)
	alternate := walk.RGB(200, 210, 220)

	if color := previewLineColor(primary, alternate, 2, 3, true); color != primary {
		t.Fatalf("expected newest line to use primary color, got %#x", uint32(color))
	}
	if color := previewLineColor(primary, alternate, 1, 3, true); color != alternate {
		t.Fatalf("expected middle line to use alternate color, got %#x", uint32(color))
	}
	if color := previewLineColor(primary, alternate, 0, 3, true); color != primary {
		t.Fatalf("expected oldest line to wrap back to primary color, got %#x", uint32(color))
	}
}
