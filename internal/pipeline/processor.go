//go:build windows

package pipeline

import (
	"context"
	"errors"
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
	RetryDelay            time.Duration
	MaxRetriesPerSnapshot int
}

// Processor implements the reference-project SyncLoop approach:
// instead of translating the full caption snapshot and diffing the results,
// it extracts the *current sentence window* (last EOS boundary → end of text)
// and translates only that.  Completed sentences (ending with an EOS terminal)
// are pushed as final history lines; in-progress partial sentences overwrite the
// single partial preview line.  This eliminates duplicates caused by LLM
// non-determinism on already-committed text.
type Processor struct {
	config     Config
	translator Translator
	output     Output

	mu            sync.Mutex
	parent        context.Context
	lastCaption   string // last extracted current-sentence window
	queued        string // next caption to translate
	queuedFinal   bool   // whether queued caption is a complete sentence
	active        string // caption currently being translated
	activeFinal   bool   // whether active caption is a complete sentence
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
		config.IdleFlushDelay = 1500 * time.Millisecond
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

	// Extract the current sentence window: last EOS boundary → end of text.
	currentCaption := extractCurrentCaption(normalized)
	if currentCaption == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.parent = parent
	if currentCaption == p.lastCaption {
		// Nothing new to translate.
		return
	}

	p.lastCaption = currentCaption
	p.retryCount = 0
	p.retryPending = false

	isFinal := isCompleteCaption(currentCaption)
	wasEmpty := p.queued == ""
	p.queued = currentCaption
	p.queuedFinal = isFinal

	if p.translating {
		if wasEmpty {
			p.firstQueued = time.Now()
		}
		// If a complete sentence just arrived and we are only translating a
		// partial, cancel the in-flight partial so we can start the final
		// sentence sooner.
		if isFinal && !p.activeFinal && p.cancel != nil {
			p.cancel()
		}
		return
	}

	if wasEmpty {
		p.firstQueued = time.Now()
	}

	if isFinal {
		// Complete sentence: start immediately without debounce.
		if p.debounceTimer != nil {
			p.debounceTimer.Stop()
			p.debounceTimer = nil
		}
		p.startNextLocked()
	} else {
		// Partial sentence: debounce to let more words accumulate.
		p.resetDebounceLocked()
	}
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

	p.parent = nil
	p.queued = ""
	p.active = ""
	p.lastCaption = ""
	p.translating = false
}

// resetDebounceLocked resets (or starts) the debounce timer.
// Must be called with p.mu held.
func (p *Processor) resetDebounceLocked() {
	if p.debounceTimer != nil {
		// Prevent infinite starvation if captions are updated continuously.
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
	isFinal := p.queuedFinal
	p.queued = ""
	p.queuedFinal = false
	p.retryPending = false

	ctx, cancel := context.WithTimeout(p.parent, p.config.RequestTimeout)
	p.cancel = cancel
	p.active = source
	p.activeFinal = isFinal
	p.translating = true

	go p.translateCaption(source, isFinal, ctx, cancel)
}

func (p *Processor) translateCaption(source string, isFinal bool, requestCtx context.Context, release context.CancelFunc) {
	defer release()

	translated, err := p.translator.Translate(requestCtx, source)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			p.finishCaption(source, isFinal, "", true, false)
			return
		}
		p.finishCaption(source, isFinal, "", false, true)
		return
	}

	translated = textutil.NormalizeCaptionSnapshot(translated)
	if translated == "" {
		translated = source
	}

	p.finishCaption(source, isFinal, translated, false, false)
}

func (p *Processor) finishCaption(source string, isFinal bool, value string, canceled bool, failed bool) {
	value = textutil.NormalizeCaptionSnapshot(value)
	shouldOutput := false
	retrySource := ""
	retryIsFinal := false
	retryDelay := time.Duration(0)

	p.mu.Lock()
	if p.active == source {
		p.active = ""
		p.activeFinal = false
	}
	if p.cancel != nil {
		p.cancel = nil
	}

	if !canceled && !failed && value != "" {
		shouldOutput = true
		if source == p.lastCaption {
			p.retryCount = 0
		}
	}

	if failed && source == p.lastCaption && p.retryCount < p.config.MaxRetriesPerSnapshot && !p.retryPending {
		p.retryCount++
		p.retryPending = true
		retrySource = source
		retryIsFinal = isFinal
		retryDelay = p.config.RetryDelay
	}

	p.translating = false

	// Schedule the next item.  Finals start immediately; partials are debounced.
	if p.queued != "" {
		if p.queuedFinal {
			p.startNextLocked()
		} else {
			p.resetDebounceLocked()
		}
	}
	p.mu.Unlock()

	if shouldOutput {
		if isFinal {
			// Completed sentence: add as a permanent history line.
			p.output.PushCaption([]string{value}, "")
		} else {
			// In-progress sentence: overwrite the partial preview line.
			p.output.PushCaption(nil, value)
		}
	}

	if retrySource != "" {
		time.AfterFunc(retryDelay, func() {
			p.mu.Lock()
			defer p.mu.Unlock()

			p.retryPending = false
			if p.parent == nil || p.translating || p.queued != "" || p.lastCaption != retrySource {
				return
			}

			p.queued = retrySource
			p.queuedFinal = retryIsFinal
			p.firstQueued = time.Now()
			p.startNextLocked()
		})
	}
}
