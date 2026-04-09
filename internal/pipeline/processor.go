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
	PushCaption(value string)
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

	mu           sync.Mutex
	parent       context.Context
	lastInput    string
	queued       string
	active       string
	translating  bool
	cancel       context.CancelFunc
	retryPending bool
	retryCount   int
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
	p.queued = normalized

	if p.translating && p.cancel != nil {
		p.cancel()
	}

	p.startNextLocked()
}

func (p *Processor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	p.queued = ""
	p.active = ""
	p.translating = false
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
	if !canceled && !failed && value != "" && source == p.lastInput {
		shouldOutput = true
		p.retryCount = 0
	}

	if failed && source == p.lastInput && p.retryCount < p.config.MaxRetriesPerSnapshot && !p.retryPending {
		p.retryCount++
		p.retryPending = true
		retrySource = source
		retryDelay = p.config.RetryDelay
	}

	p.translating = false
	p.startNextLocked()
	p.mu.Unlock()

	if shouldOutput {
		p.output.PushCaption(value)
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
			p.startNextLocked()
		})
	}
}
