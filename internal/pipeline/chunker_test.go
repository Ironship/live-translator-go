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

func TestChunksDeltaEmptyCommitted(t *testing.T) {
	delta := chunksDelta(nil, []string{"Hello.", "World."})
	if len(delta) != 2 || delta[0] != "Hello." || delta[1] != "World." {
		t.Fatalf("expected all incoming as delta, got %#v", delta)
	}
}

func TestChunksDeltaFullOverlap(t *testing.T) {
	committed := []string{"Hello.", "World."}
	delta := chunksDelta(committed, []string{"Hello.", "World."})
	if len(delta) != 0 {
		t.Fatalf("expected empty delta for identical sets, got %#v", delta)
	}
}

func TestChunksDeltaPartialOverlap(t *testing.T) {
	committed := []string{"Hello.", "World."}
	delta := chunksDelta(committed, []string{"Hello.", "World.", "New sentence."})
	if len(delta) != 1 || delta[0] != "New sentence." {
		t.Fatalf("expected one new chunk, got %#v", delta)
	}
}

func TestChunksDeltaScrollForward(t *testing.T) {
	committed := []string{"A.", "B.", "C."}
	delta := chunksDelta(committed, []string{"B.", "C.", "D."})
	if len(delta) != 1 || delta[0] != "D." {
		t.Fatalf("expected one new chunk after scroll, got %#v", delta)
	}
}

func TestChunksDeltaTopicChange(t *testing.T) {
	committed := []string{"Hello.", "World."}
	delta := chunksDelta(committed, []string{"Completely.", "Different."})
	if len(delta) != 2 || delta[0] != "Completely." || delta[1] != "Different." {
		t.Fatalf("expected all incoming for topic change, got %#v", delta)
	}
}

func TestChunksDeltaNilIncoming(t *testing.T) {
	committed := []string{"Hello."}
	delta := chunksDelta(committed, nil)
	if delta != nil {
		t.Fatalf("expected nil delta for nil incoming, got %#v", delta)
	}
}

