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

	mu              sync.Mutex
	parent          context.Context
	lastInput       string
	pendingSource   string
	committedAnchor string
	queue           []string
	translating     bool
	cancel          context.CancelFunc
	idleTimer       *time.Timer
	idleVersion     uint64
}

func NewProcessor(config Config, translator Translator, output Output) *Processor {
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 8 * time.Second
	}
	if config.IdleFlushDelay <= 0 {
		config.IdleFlushDelay = 900 * time.Millisecond
	}
	if config.ForceChunkWords <= 0 {
		config.ForceChunkWords = 16
	}
	if config.ForceChunkChars <= 0 {
		config.ForceChunkChars = 110
	}
	if config.ForceChunkAnchorWords <= 0 {
		config.ForceChunkAnchorWords = 4
	}

	return &Processor{
		config:     config,
		translator: translator,
		output:     output,
	}
}

func (p *Processor) Submit(parent context.Context, input string) {
	normalized := textutil.NormalizeCaption(input)
	if normalized == "" {
		return
	}

	p.mu.Lock()
	p.parent = parent
	if normalized == p.lastInput {
		p.mu.Unlock()
		return
	}

	p.lastInput = normalized
	if p.pendingSource == "" {
		p.pendingSource = pendingFromCurrentAfterAnchor(p.committedAnchor, normalized)
	} else {
		updatedPending, reset := mergePendingSource(p.pendingSource, normalized)
		if reset {
			p.enqueueLocked(p.pendingSource)
		}
		p.pendingSource = updatedPending
	}

	readyChunks, remainder := consumeReadyChunks(p.pendingSource, p.config)
	p.pendingSource = remainder
	for _, chunk := range readyChunks {
		p.enqueueLocked(chunk)
	}
	p.scheduleIdleFlushLocked()
	p.startNextLocked()
	p.mu.Unlock()
}

func (p *Processor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}
	p.queue = nil
	p.pendingSource = ""
}

func (p *Processor) enqueueLocked(value string) {
	chunk := textutil.NormalizeCaption(value)
	if chunk == "" {
		return
	}

	p.queue = append(p.queue, chunk)
	p.committedAnchor = trailingAnchor(chunk, p.config.ForceChunkAnchorWords)
}

func (p *Processor) scheduleIdleFlushLocked() {
	p.idleVersion++
	if p.idleTimer != nil {
		p.idleTimer.Stop()
		p.idleTimer = nil
	}
	if p.pendingSource == "" {
		return
	}

	version := p.idleVersion
	p.idleTimer = time.AfterFunc(p.config.IdleFlushDelay, func() {
		p.flushPending(version)
	})
}

func (p *Processor) flushPending(version uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if version != p.idleVersion || p.pendingSource == "" {
		return
	}

	p.enqueueLocked(p.pendingSource)
	p.pendingSource = ""
	p.idleTimer = nil
	p.startNextLocked()
}

func (p *Processor) startNextLocked() {
	if p.translating || len(p.queue) == 0 || p.parent == nil {
		return
	}

	source := p.queue[0]
	p.queue = p.queue[1:]
	ctx, cancel := context.WithTimeout(p.parent, p.config.RequestTimeout)
	p.cancel = cancel
	p.translating = true

	go p.translateChunk(source, ctx, cancel)
}

func (p *Processor) translateChunk(source string, requestCtx context.Context, release context.CancelFunc) {
	defer release()

	translated, err := p.translator.Translate(requestCtx, source)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			p.finishChunk("")
			return
		}

		p.finishChunk(source)
		return
	}

	translated = textutil.NormalizeCaption(translated)
	if translated == "" {
		translated = source
	}

	p.finishChunk(translated)
}

func (p *Processor) finishChunk(value string) {
	if value != "" {
		p.output.PushCaption(value)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel = nil
	}
	p.translating = false
	p.startNextLocked()
}
