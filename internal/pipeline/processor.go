//go:build windows

package pipeline

import (
	"context"
	"sync"
	"time"

	textutil "live-translator-go/internal/text"
)

type Translator interface {
	Translate(ctx context.Context, input string) (string, error)
}

type Output interface {
	PushCaption(finalChunks []string, partialChunk string)
}

type Config struct {
	RequestTimeout        time.Duration
	IdleFlushDelay        time.Duration
	ForceChunkWords       int
	ForceChunkChars       int
	ForceChunkAnchorWords int
	RetryDelay            time.Duration
	MaxRetriesPerSnapshot int
}

type Processor struct {
	config     Config
	translator Translator
	output     Output

	mu            sync.Mutex
	parent        context.Context
	lastInput     string
	queued        string
	active        string
	translating   bool
	cancel        context.CancelFunc
	retryPending  bool
	retryCount    int
	debounceTimer *time.Timer
	firstQueued   time.Time
}

func NewProcessor(config Config, translator Translator, output Output) *Processor {
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 8 * time.Second
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 800 * time.Millisecond
	}
	if config.MaxRetriesPerSnapshot <= 0 {
		config.MaxRetriesPerSnapshot = 2
	}
	if config.IdleFlushDelay <= 0 {
		config.IdleFlushDelay = 300 * time.Millisecond
	}

	return &Processor{
		config:     config,
		translator: translator,
		output:     output,
	}
}

func (p *Processor) Submit(parent context.Context, input string) {
	normalized := textutil.NormalizeCaptionSnapshot(input)
	if normalized == "" {
		return
	}

	normalized = extractCurrentCaption(normalized)
	if normalized == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.parent = parent
	if normalized == p.lastInput {
		return
	}

	p.lastInput = normalized
	p.retryCount = 0
	p.retryPending = false

	wasEmpty := p.queued == ""
	p.queued = normalized

	// If a translation is already in progress, just queue the text.
	// When it finishes, startNextLocked() will pick up the latest.
	if p.translating {
		if wasEmpty {
			p.firstQueued = time.Now()
		}
		// Newer snapshot should preempt stale in-flight translation work.
		if p.active != "" && p.active != normalized && p.cancel != nil {
			p.cancel()
		}
		return
	}

	if wasEmpty {
		p.firstQueued = time.Now()
	}

	p.resetDebounceLocked()
}

func (p *Processor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
		p.debounceTimer = nil
	}

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	p.queued = ""
	p.active = ""
	p.translating = false
}
