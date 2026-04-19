//go:build windows

package pipeline

import (
	"reflect"
	"testing"
)

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

// TestIsCompleteCaptionDoesNotTreatAbbreviationAsComplete verifies that a
// partial caption ending in a common abbreviation is NOT flushed early —
// otherwise the pipeline would start translating "said Mr." as if the
// speaker had finished a sentence.
func TestIsCompleteCaptionDoesNotTreatAbbreviationAsComplete(t *testing.T) {
	cases := []string{
		"I saw Mr.",
		"Go to St.",
		"e.g.",
		"at 10 a.m.",
		"Dr.",
	}
	for _, c := range cases {
		if isCompleteCaption(c) {
			t.Errorf("expected %q to be treated as incomplete", c)
		}
	}
}

// TestConsumeSentenceChunksSkipsAbbreviations guards the main duplicate-causing
// bug: splitting on "Mr. Smith" would emit "Mr." as a committed sentence,
// which then wouldn't match any re-emission of the caption buffer.
func TestConsumeSentenceChunksSkipsAbbreviations(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		chunks    []string
		remainder string
	}{
		{
			name:      "title abbreviation mid-sentence",
			input:     "Mr. Smith arrived late.",
			chunks:    []string{"Mr. Smith arrived late."},
			remainder: "",
		},
		{
			name:      "two titles in one sentence",
			input:     "Dr. Jones and Mrs. Lee met.",
			chunks:    []string{"Dr. Jones and Mrs. Lee met."},
			remainder: "",
		},
		{
			name:      "e.g. list style",
			input:     "Many items, e.g. apples, pears.",
			chunks:    []string{"Many items, e.g. apples, pears."},
			remainder: "",
		},
		{
			name:      "a.m. time",
			input:     "We meet at 10 a.m. tomorrow.",
			chunks:    []string{"We meet at 10 a.m. tomorrow."},
			remainder: "",
		},
		{
			name:      "real sentence boundary still splits",
			input:     "Mr. Smith arrived. He was late.",
			chunks:    []string{"Mr. Smith arrived.", "He was late."},
			remainder: "",
		},
		{
			name:      "partial ending in abbreviation is remainder",
			input:     "I saw Mr.",
			chunks:    nil,
			remainder: "I saw Mr.",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chunks, remainder := consumeSentenceChunks(c.input)
			if len(chunks) == 0 && len(c.chunks) == 0 {
				// Normalize both to nil for comparison; `make([]string, 0, N)` vs nil.
			} else if !reflect.DeepEqual(chunks, c.chunks) {
				t.Errorf("chunks mismatch: got %q, want %q", chunks, c.chunks)
			}
			if remainder != c.remainder {
				t.Errorf("remainder mismatch: got %q, want %q", remainder, c.remainder)
			}
		})
	}
}

// TestPendingFromCurrentAfterAnchorCaseAndPunctuationInsensitive verifies that
// anchor matching survives Live Captions re-wrapping the buffer with different
// casing or comma placement — which was previously a source of re-translation
// and therefore duplicate output lines.
func TestPendingFromCurrentAfterAnchorCaseAndPunctuationInsensitive(t *testing.T) {
	cases := []struct {
		name    string
		anchor  string
		current string
		want    string
	}{
		{
			name:    "case change at sentence boundary",
			anchor:  "hello world.",
			current: "Hello world. How are you?",
			want:    "How are you?",
		},
		{
			name:    "trailing comma dropped in new snapshot",
			anchor:  "Hello, world.",
			current: "Hello world. How are you?",
			want:    "How are you?",
		},
		{
			name:    "comma added in new snapshot",
			anchor:  "Hello world.",
			current: "Hello, world. Next sentence here.",
			want:    "Next sentence here.",
		},
		{
			name:    "fully consumed",
			anchor:  "Hello world.",
			current: "hello world.",
			want:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pendingFromCurrentAfterAnchor(c.anchor, c.current)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestCanonicalizeToken(t *testing.T) {
	cases := map[string]string{
		"Hello,":   "hello",
		"WORLD.":   "world.", // sentence-terminating period is significant
		"Mr.":      "mr.",
		"end—":     "end",
		"End;":     "end",
		"plain":    "plain",
		"":         "",
		"Hello,,,": "hello",
	}
	for input, want := range cases {
		if got := canonicalizeToken(input); got != want {
			t.Errorf("canonicalizeToken(%q)=%q, want %q", input, got, want)
		}
	}
}
