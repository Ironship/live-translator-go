//go:build windows

package pipeline

import "testing"

func TestIsCompleteCaptionTrue(t *testing.T) {
	if !isCompleteCaption("Hello world.") {
		t.Fatal("expected true for sentence ending with '.'")
	}
	if !isCompleteCaption("Really?") {
		t.Fatal("expected true for sentence ending with '?'")
	}
}

func TestIsCompleteCaptionFalse(t *testing.T) {
	if isCompleteCaption("Hello world") {
		t.Fatal("expected false for sentence without EOS terminal")
	}
	if isCompleteCaption("") {
		t.Fatal("expected false for empty string")
	}
}
