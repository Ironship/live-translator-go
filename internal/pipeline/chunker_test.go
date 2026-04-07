//go:build windows

package pipeline

import "testing"

func TestConsumeSentenceChunks(t *testing.T) {
	chunks, remainder := consumeSentenceChunks("Hello there. General Kenobi! still waiting")
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello there." {
		t.Fatalf("unexpected first chunk: %q", chunks[0])
	}
	if chunks[1] != "General Kenobi!" {
		t.Fatalf("unexpected second chunk: %q", chunks[1])
	}
	if remainder != "still waiting" {
		t.Fatalf("unexpected remainder: %q", remainder)
	}
}

func TestSplitForcedChunkLeavesAnchorTail(t *testing.T) {
	chunk, remainder := splitForcedChunk("one two three four five six seven eight", 6, 999, 2)
	if chunk != "one two three four five six" {
		t.Fatalf("unexpected chunk: %q", chunk)
	}
	if remainder != "seven eight" {
		t.Fatalf("unexpected remainder: %q", remainder)
	}
}

func TestPendingFromCurrentAfterAnchor(t *testing.T) {
	pending := pendingFromCurrentAfterAnchor("hello world.", "hello world. how are you")
	if pending != "how are you" {
		t.Fatalf("unexpected pending tail: %q", pending)
	}
}

func TestMergePendingSourceWithVisibleCommittedPrefix(t *testing.T) {
	merged, reset := mergePendingSource("second sentence start", "hello there. second sentence start more words")
	if reset {
		t.Fatalf("expected merge without reset")
	}
	if merged != "second sentence start more words" {
		t.Fatalf("unexpected merged value: %q", merged)
	}
}

func TestMergePendingSourceKeepsCorrectionContext(t *testing.T) {
	merged, reset := mergePendingSource("i think we should too", "i think we should do that")
	if reset {
		t.Fatalf("expected merge without reset")
	}
	if merged != "i think we should do that" {
		t.Fatalf("unexpected merged correction: %q", merged)
	}
}

func TestMergePendingSourceFallsBackToReset(t *testing.T) {
	merged, reset := mergePendingSource("i think we should", "new unrelated topic")
	if !reset {
		t.Fatalf("expected reset")
	}
	if merged != "new unrelated topic" {
		t.Fatalf("unexpected reset value: %q", merged)
	}
}
