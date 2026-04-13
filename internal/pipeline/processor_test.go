//go:build windows

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// outputCall records a single PushCaption invocation.
type outputCall struct {
	finalChunks  []string
	partialChunk string
}

type recordingOutput struct {
	mu    sync.Mutex
	calls []outputCall
	ch    chan outputCall
}

func newRecordingOutput() *recordingOutput {
	return &recordingOutput{ch: make(chan outputCall, 16)}
}

func (r *recordingOutput) PushCaption(finalChunks []string, partialChunk string) {
	call := outputCall{finalChunks: append([]string(nil), finalChunks...), partialChunk: partialChunk}
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	r.ch <- call
}

func (r *recordingOutput) snapshot() []outputCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]outputCall(nil), r.calls...)
}

// fastConfig returns a Config suitable for unit tests: tiny debounce delay so
// partial captions start translating quickly, and realistic timeouts.
func fastConfig() Config {
	return Config{
		RequestTimeout:        2 * time.Second,
		IdleFlushDelay:        20 * time.Millisecond,
		RetryDelay:            25 * time.Millisecond,
		MaxRetriesPerSnapshot: 2,
	}
}

type controllableTranslator struct {
	mu      sync.Mutex
	calls   []string
	release map[string]chan struct{}
}

func newControllableTranslator() *controllableTranslator {
	return &controllableTranslator{release: make(map[string]chan struct{})}
}

func (t *controllableTranslator) allow(input string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch, ok := t.release[input]
	if !ok {
		ch = make(chan struct{})
		t.release[input] = ch
	}
	// Only close once.
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func (t *controllableTranslator) Translate(ctx context.Context, input string) (string, error) {
	t.mu.Lock()
	t.calls = append(t.calls, input)
	ch, ok := t.release[input]
	if !ok {
		ch = make(chan struct{})
		t.release[input] = ch
	}
	t.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-ch:
		return fmt.Sprintf("tr:%s", input), nil
	}
}

// TestProcessorSkipsDuplicateInput verifies that submitting the same text twice
// results in only one translator call.
func TestProcessorSkipsDuplicateInput(t *testing.T) {
	out := newRecordingOutput()
	tr := newControllableTranslator()
	p := NewProcessor(fastConfig(), tr, out)
	defer p.Close()

	tr.allow("hello")
	p.Submit(context.Background(), "hello")
	p.Submit(context.Background(), "hello")

	select {
	case <-out.ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for output")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.calls) != 1 {
		t.Fatalf("expected one translator call, got %d", len(tr.calls))
	}
}

// TestProcessorPartialEmitsPartialChunk verifies that a caption without an EOS
// terminal is emitted via PushCaption as a partialChunk (nil finalChunks).
func TestProcessorPartialEmitsPartialChunk(t *testing.T) {
	out := newRecordingOutput()
	tr := newControllableTranslator()
	p := NewProcessor(fastConfig(), tr, out)
	defer p.Close()

	tr.allow("hello world")
	p.Submit(context.Background(), "hello world") // no EOS → partial

	select {
	case call := <-out.ch:
		if len(call.finalChunks) != 0 {
			t.Fatalf("expected no final chunks for partial caption, got %v", call.finalChunks)
		}
		if call.partialChunk != "tr:hello world" {
			t.Fatalf("expected partialChunk %q, got %q", "tr:hello world", call.partialChunk)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for partial output")
	}
}

// TestProcessorCompleteEmitsFinalChunk verifies that a caption ending with an
// EOS terminal is emitted via PushCaption as a finalChunk (empty partialChunk).
func TestProcessorCompleteEmitsFinalChunk(t *testing.T) {
	out := newRecordingOutput()
	tr := newControllableTranslator()
	p := NewProcessor(fastConfig(), tr, out)
	defer p.Close()

	tr.allow("Hello world.")
	p.Submit(context.Background(), "Hello world.") // ends with '.' → complete

	select {
	case call := <-out.ch:
		if len(call.finalChunks) != 1 || call.finalChunks[0] != "tr:Hello world." {
			t.Fatalf("expected finalChunks=[%q], got %v", "tr:Hello world.", call.finalChunks)
		}
		if call.partialChunk != "" {
			t.Fatalf("expected empty partialChunk, got %q", call.partialChunk)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for final output")
	}
}

// TestProcessorNoDuplicatesOnLLMRephrase is the core regression test.
// Even if the LLM translates older sentences differently across calls, we should
// never see duplicate entries because each sentence is only translated once.
func TestProcessorNoDuplicatesOnLLMRephrase(t *testing.T) {
	out := newRecordingOutput()
	// Translator always returns the input prefixed with "tr:" — simulates stable LLM.
	tr := newControllableTranslator()
	p := NewProcessor(fastConfig(), tr, out)
	defer p.Close()

	// First complete sentence.
	tr.allow("Hello world.")
	p.Submit(context.Background(), "Hello world.")
	select {
	case call := <-out.ch:
		if len(call.finalChunks) != 1 {
			t.Fatalf("expected 1 final chunk, got %v", call.finalChunks)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out on first sentence")
	}

	// Second complete sentence appended in the LiveCaptions buffer.
	// extractCurrentCaption picks up only "How are you?" — the first sentence
	// is never re-translated.
	tr.allow("How are you?")
	p.Submit(context.Background(), "Hello world.\nHow are you?")
	select {
	case call := <-out.ch:
		if len(call.finalChunks) != 1 || call.finalChunks[0] != "tr:How are you?" {
			t.Fatalf("expected second sentence as single final chunk, got %v", call.finalChunks)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out on second sentence")
	}

	// Exactly two output calls — no duplicates from any rephrasing.
	calls := out.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected exactly 2 output calls, got %d: %+v", len(calls), calls)
	}
}

// TestProcessorCompletePreemptsInFlightPartial verifies that when a complete
// sentence arrives while a partial is being translated, the partial is cancelled
// and the complete sentence is translated instead.
func TestProcessorCompletePreemptsInFlightPartial(t *testing.T) {
	out := newRecordingOutput()
	tr := newControllableTranslator()
	p := NewProcessor(fastConfig(), tr, out)
	defer p.Close()

	// Submit partial — it starts translating after debounce.
	p.Submit(context.Background(), "Hello world")
	// Give debounce a moment to fire and start the partial translation.
	time.Sleep(50 * time.Millisecond)

	// Now submit a complete sentence — the partial in flight should be cancelled.
	tr.allow("Hello world.")
	p.Submit(context.Background(), "Hello world.")

	// Don't allow the partial translation to finish (it's cancelled).
	select {
	case call := <-out.ch:
		// We only expect a final chunk for the complete sentence.
		if len(call.finalChunks) != 1 || call.finalChunks[0] != "tr:Hello world." {
			t.Fatalf("expected final chunk for complete sentence, got %+v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for complete sentence output")
	}
}

type retryTranslator struct {
	mu    sync.Mutex
	calls int
}

func (t *retryTranslator) Translate(ctx context.Context, input string) (string, error) {
	t.mu.Lock()
	t.calls++
	callNo := t.calls
	t.mu.Unlock()

	if callNo == 1 {
		return "", errors.New("temporary upstream failure")
	}

	return fmt.Sprintf("tr:%s", input), nil
}

func TestProcessorRetriesOnFailureWithoutOutputtingSource(t *testing.T) {
	out := newRecordingOutput()
	tr := &retryTranslator{}
	p := NewProcessor(fastConfig(), tr, out)
	defer p.Close()

	p.Submit(context.Background(), "hello world")

	select {
	case call := <-out.ch:
		combined := call.partialChunk
		if len(call.finalChunks) > 0 {
			combined = call.finalChunks[0]
		}
		if combined != "tr:hello world" {
			t.Fatalf("expected translated output after retry, got %q", combined)
		}
		if combined == "hello world" {
			t.Fatal("unexpected raw source fallback output")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for retried translation output")
	}

	calls := out.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one output after retry recovery, got %d (%+v)", len(calls), calls)
	}
}

func TestProcessorStartsImmediatelyWhenSentenceIsComplete(t *testing.T) {
	out := newRecordingOutput()
	translator := newControllableTranslator()
	processor := NewProcessor(Config{
		RequestTimeout: 2 * time.Second,
		IdleFlushDelay: 1200 * time.Millisecond,
	}, translator, out)
	defer processor.Close()

	processor.Submit(context.Background(), "hello world.")

	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		translator.mu.Lock()
		callCount := len(translator.calls)
		first := ""
		if callCount > 0 {
			first = translator.calls[0]
		}
		translator.mu.Unlock()

		if callCount > 0 {
			if first != "hello world." {
				t.Fatalf("unexpected first translator input: %q", first)
			}
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("translator did not start immediately for complete sentence")
		}
		time.Sleep(10 * time.Millisecond)
	}

	translator.allow("hello world.")
	select {
	case call := <-out.ch:
		if len(call.finalChunks) != 1 || call.finalChunks[0] != "tr:hello world." {
			t.Fatalf("unexpected final output: %+v", call)
		}
		if call.partialChunk != "" {
			t.Fatalf("expected empty partial chunk, got: %q", call.partialChunk)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for output")
	}
}
