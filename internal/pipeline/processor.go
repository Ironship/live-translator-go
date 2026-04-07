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
}

type Processor struct {
	config     Config
	translator Translator
	output     Output

	mu          sync.Mutex
	parent      context.Context
	lastInput   string
	queued      string
	active      string
	translating bool
	cancel      context.CancelFunc
}

func NewProcessor(config Config, translator Translator, output Output) *Processor {
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 8 * time.Second
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
			p.finishSnapshot(source, "", true)
			return
		}

		p.finishSnapshot(source, source, false)
		return
	}

	translated = textutil.NormalizeCaptionSnapshot(translated)
	if translated == "" {
		translated = source
	}

	p.finishSnapshot(source, translated, false)
}

func (p *Processor) finishSnapshot(source string, value string, canceled bool) {
	value = textutil.NormalizeCaptionSnapshot(value)
	shouldOutput := false

	p.mu.Lock()
	if p.active == source {
		p.active = ""
	}

	if p.cancel != nil {
		p.cancel = nil
	}
	if !canceled && value != "" && source == p.lastInput {
		shouldOutput = true
	}

	p.translating = false
	p.startNextLocked()
	p.mu.Unlock()

	if shouldOutput {
		p.output.PushCaption(value)
	}
}
