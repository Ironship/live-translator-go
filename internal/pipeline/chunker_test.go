//go:build windows

package pipeline

import "testing"

func TestExtractCurrentCaptionPartial(t *testing.T) {
	got := extractCurrentCaption("Hello world. How are you")
	if got != "How are you" {
		t.Fatalf("expected %q, got %q", "How are you", got)
	}
}

func TestExtractCurrentCaptionCompleteLastSentence(t *testing.T) {
	got := extractCurrentCaption("Hello world. How are you?")
	if got != "How are you?" {
		t.Fatalf("expected %q, got %q", "How are you?", got)
	}
}

func TestExtractCurrentCaptionOnlyOneSentence(t *testing.T) {
	got := extractCurrentCaption("Hello world?")
	if got != "Hello world?" {
		t.Fatalf("expected %q, got %q", "Hello world?", got)
	}
}

func TestExtractCurrentCaptionNoEOS(t *testing.T) {
	got := extractCurrentCaption("Hello world")
	if got != "Hello world" {
		t.Fatalf("expected %q, got %q", "Hello world", got)
	}
}

func TestExtractCurrentCaptionMultiLine(t *testing.T) {
	// Newlines are flattened to spaces; last sentence window is extracted.
	got := extractCurrentCaption("Hello world.\nHow are you?")
	if got != "How are you?" {
		t.Fatalf("expected %q, got %q", "How are you?", got)
	}
}

func TestExtractCurrentCaptionShortExtension(t *testing.T) {
	// "Hi." is very short (< 10 bytes) so we extend back to "Nice. Hi."
	got := extractCurrentCaption("Long sentence here. Nice. Hi.")
	if got != "Nice. Hi." {
		t.Fatalf("expected extended window %q, got %q", "Nice. Hi.", got)
	}
}

func TestExtractCurrentCaptionEmpty(t *testing.T) {
	got := extractCurrentCaption("")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

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
