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

type recordingOutput struct {
	mu     sync.Mutex
	values []string
	ch     chan string
}

func newRecordingOutput() *recordingOutput {
	return &recordingOutput{ch: make(chan string, 8)}
}

func (r *recordingOutput) PushCaption(finalChunks []string, partialChunk string) {
	var combined []string
	combined = append(combined, finalChunks...)
	if partialChunk != "" {
		combined = append(combined, partialChunk)
	}

	value := ""
	for i, c := range combined {
		if i > 0 {
			value += " "
		}
		value += c
	}

	r.mu.Lock()
	r.values = append(r.values, value)
	r.mu.Unlock()
	r.ch <- value
}

func (r *recordingOutput) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.values...)
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
	close(ch)
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

func TestProcessorSkipsDuplicateInput(t *testing.T) {
	out := newRecordingOutput()
	translator := newControllableTranslator()
	processor := NewProcessor(Config{RequestTimeout: 2 * time.Second}, translator, out)
	defer processor.Close()

	translator.allow("hello")
	processor.Submit(context.Background(), "hello")
	processor.Submit(context.Background(), "hello")

	select {
	case <-out.ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for output")
	}

	translator.mu.Lock()
	defer translator.mu.Unlock()
	if len(translator.calls) != 1 {
		t.Fatalf("expected one translator call, got %d", len(translator.calls))
	}
}

func TestProcessorOutputsOnlyLatestSnapshot(t *testing.T) {
	out := newRecordingOutput()
	translator := newControllableTranslator()
	processor := NewProcessor(Config{RequestTimeout: 2 * time.Second}, translator, out)
	defer processor.Close()

	processor.Submit(context.Background(), "old snapshot")
	processor.Submit(context.Background(), "new snapshot")

	// Allow both: "old snapshot" completes first, its output is discarded
	// (source != lastInput), then "new snapshot" runs and produces output.
	translator.allow("old snapshot")
	translator.allow("new snapshot")

	select {
	case got := <-out.ch:
		if got != "tr:new snapshot" {
			t.Fatalf("expected latest snapshot output, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for latest output")
	}

	if values := out.snapshot(); len(values) != 1 {
		t.Fatalf("expected exactly one output value, got %d (%#v)", len(values), values)
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
	translator := &retryTranslator{}
	processor := NewProcessor(Config{
		RequestTimeout:        2 * time.Second,
		RetryDelay:            25 * time.Millisecond,
		MaxRetriesPerSnapshot: 2,
	}, translator, out)
	defer processor.Close()

	processor.Submit(context.Background(), "hello world")

	select {
	case got := <-out.ch:
		if got != "tr:hello world" {
			t.Fatalf("expected translated output after retry, got %q", got)
		}
		if got == "hello world" {
			t.Fatalf("unexpected raw source fallback output")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for retried translation output")
	}

	if values := out.snapshot(); len(values) != 1 {
		t.Fatalf("expected exactly one output after retry recovery, got %d (%#v)", len(values), values)
	}
}
