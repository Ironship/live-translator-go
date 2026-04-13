//go:build windows

package pipeline

import (
	"context"
	"errors"
	"strings"
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
		return
	}

	if wasEmpty {
		p.firstQueued = time.Now()
	}

	if shouldTranslateImmediately(normalized) {
		if p.debounceTimer != nil {
			p.debounceTimer.Stop()
			p.debounceTimer = nil
		}
		p.startNextLocked()
		return
	}

	// Debounce: wait for text to stabilize before translating.
	// Live Captions sends a new snapshot every ~0.5s as words are added;
	// this prevents flooding the LLM with near-identical requests.
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

// resetDebounceLocked resets (or starts) the debounce timer.
// Must be called with p.mu held.
func (p *Processor) resetDebounceLocked() {
	if p.debounceTimer != nil {
		// Prevent infinite starvation if captions are updated continuously.
		// If we've been queued for more than 2.5x the flush delay without translating, let the existing timer run.
		maxWait := p.config.IdleFlushDelay*2 + (p.config.IdleFlushDelay / 2)
		if time.Since(p.firstQueued) >= maxWait {
			return
		}
		p.debounceTimer.Stop()
	}
	p.debounceTimer = time.AfterFunc(p.config.IdleFlushDelay, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.debounceTimer = nil
		p.startNextLocked()
	})
}

func (p *Processor) startNextLocked() {
	if p.translating || p.parent == nil || p.queued == "" {
		return
	}

	source := p.queued
	p.queued = ""
	p.retryPending = false

	ctx, cancel := context.WithTimeout(p.parent, p.config.RequestTimeout)
	p.cancel = cancel
	p.active = source
	p.translating = true

	go p.translateSnapshot(source, ctx, cancel)
}

func (p *Processor) translateSnapshot(source string, requestCtx context.Context, release context.CancelFunc) {
	defer release()

	translated, err := p.translator.Translate(requestCtx, source)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			p.finishSnapshot(source, "", true, false)
			return
		}

		p.finishSnapshot(source, "", false, true)
		return
	}

	translated = textutil.NormalizeCaptionSnapshot(translated)
	if translated == "" {
		translated = source
	}

	p.finishSnapshot(source, translated, false, false)
}

func (p *Processor) finishSnapshot(source string, value string, canceled bool, failed bool) {
	value = textutil.NormalizeCaptionSnapshot(value)
	shouldOutput := false
	retrySource := ""
	retryDelay := time.Duration(0)

	p.mu.Lock()
	if p.active == source {
		p.active = ""
	}

	if p.cancel != nil {
		p.cancel = nil
	}
	if !canceled && !failed && value != "" {
		shouldOutput = true
		if source == p.lastInput {
			p.retryCount = 0
		}
	}

	if failed && source == p.lastInput && p.retryCount < p.config.MaxRetriesPerSnapshot && !p.retryPending {
		p.retryCount++
		p.retryPending = true
		retrySource = source
		retryDelay = p.config.RetryDelay
	}

	p.translating = false

	// If there's new text queued, debounce before starting the next
	// translation to let more words accumulate from Live Captions.
	if p.queued != "" {
		if shouldTranslateImmediately(p.queued) {
			p.startNextLocked()
		} else {
			p.resetDebounceLocked()
		}
	}
	p.mu.Unlock()

	if shouldOutput {
		chunks, remainder := consumeSentenceChunks(value)
		p.output.PushCaption(chunks, remainder)
	}

	if retrySource != "" {
		time.AfterFunc(retryDelay, func() {
			p.mu.Lock()
			defer p.mu.Unlock()

			p.retryPending = false
			if p.parent == nil || p.translating || p.queued != "" || p.lastInput != retrySource {
				return
			}

			p.queued = retrySource
			p.firstQueued = time.Now()
			p.startNextLocked()
		})
	}
}

func shouldTranslateImmediately(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	runes := []rune(trimmed)
	index := len(runes) - 1
	for index >= 0 && isSentenceTrailingRune(runes[index]) {
		index--
	}
	if index < 0 {
		return false
	}

	return isSentenceTerminal(runes[index])
}
