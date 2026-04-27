//go:build windows

package overlay

import (
	"testing"

	"github.com/lxn/walk"
)

func TestSavedBoundsLookCompactAcceptsCompactHeight(t *testing.T) {
	compactDefault := walk.Rectangle{Width: 940, Height: 252}
	saved := walk.Rectangle{Width: 980, Height: 360}

	if !savedBoundsLookCompact(saved, compactDefault) {
		t.Fatalf("expected saved bounds to be treated as compact")
	}
}

func TestSavedBoundsLookCompactRejectsSettingsHeight(t *testing.T) {
	compactDefault := walk.Rectangle{Width: 940, Height: 252}
	saved := walk.Rectangle{Width: 1080, Height: 720}

	if savedBoundsLookCompact(saved, compactDefault) {
		t.Fatalf("expected settings-sized bounds not to restore into compact mode")
	}
}

func TestLatestStoredBoundsPrefersCollapsedBoundsWhenCompact(t *testing.T) {
	window := &Window{
		settingsVisible: false,
		collapsedBounds: walk.Rectangle{X: 20, Y: 30, Width: 940, Height: 252},
		expandedBounds:  walk.Rectangle{X: 40, Y: 50, Width: 1080, Height: 720},
	}

	bounds := window.latestStoredBounds()
	if bounds != window.collapsedBounds {
		t.Fatalf("expected compact bounds, got %+v", bounds)
	}
}

func TestLatestStoredBoundsPrefersExpandedBoundsWhenSettingsVisible(t *testing.T) {
	window := &Window{
		settingsVisible: true,
		collapsedBounds: walk.Rectangle{X: 20, Y: 30, Width: 940, Height: 252},
		expandedBounds:  walk.Rectangle{X: 40, Y: 50, Width: 1080, Height: 720},
	}

	bounds := window.latestStoredBounds()
	if bounds != window.expandedBounds {
		t.Fatalf("expected settings bounds, got %+v", bounds)
	}
}
