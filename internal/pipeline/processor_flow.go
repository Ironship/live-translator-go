//go:build windows

package pipeline

import (
	"context"
	"errors"
	"time"

	textutil "live-translator-go/internal/text"
)

func (p *Processor) resetDebounceLocked() {
	if p.debounceTimer != nil {
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
	shouldOutput, retrySource, retryDelay, outputValue := p.finishSnapshotState(source, value, canceled, failed)

	if shouldOutput {
		chunks, remainder := consumeSentenceChunks(outputValue)
		p.output.PushCaption(chunks, remainder)
	}

	if retrySource != "" {
		p.scheduleRetry(retrySource, retryDelay)
	}
}

func (p *Processor) finishSnapshotState(source string, value string, canceled bool, failed bool) (bool, string, time.Duration, string) {
	normalized := textutil.NormalizeCaptionSnapshot(value)
	shouldOutput := false
	retrySource := ""
	retryDelay := time.Duration(0)

	p.mu.Lock()
	if p.active == source {
		p.active = ""
	}

	p.cancel = nil

	if !canceled && !failed && normalized != "" {
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

	if p.queued != "" {
		if canceled {
			p.startNextLocked()
		} else {
			p.resetDebounceLocked()
		}
	}
	p.mu.Unlock()

	return shouldOutput, retrySource, retryDelay, normalized
}

func (p *Processor) scheduleRetry(retrySource string, retryDelay time.Duration) {
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
