//go:build windows

package pipeline

// Pipeline package is Windows-only because the app integrates with Windows Live Captions.

import (
	"context"
	"errors"
	"time"

	textutil "live-translator-go/internal/text"
)

func (p *Processor) resetDebounceLocked() {
	if isCompleteCaption(p.queued) {
		if p.debounceTimer != nil {
			p.debounceTimer.Stop()
			p.debounceTimer = nil
		}
		p.startNextLocked()
		return
	}

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
	shouldOutput, retrySource, retryDelay, chunks, remainder := p.computeSnapshotOutcome(source, value, canceled, failed)

	if shouldOutput {
		p.output.PushCaption(chunks, remainder)
	}

	if retrySource != "" {
		p.scheduleRetry(retrySource, retryDelay)
	}
}

func (p *Processor) computeSnapshotOutcome(source string, value string, canceled bool, failed bool) (shouldOutput bool, retrySource string, retryDelay time.Duration, chunks []string, remainder string) {
	outputValue := textutil.NormalizeCaptionSnapshot(value)

	p.mu.Lock()
	if p.active == source {
		p.active = ""
	}

	if p.cancel != nil {
		p.cancel = nil
	}

	if !canceled && !failed && outputValue != "" {
		shouldOutput = true
		if source == p.lastInput {
			p.retryCount = 0
		}

		outputChunks, outputRemainder := consumeSentenceChunks(outputValue)
		sourceChunks, sourceRemainder := consumeSentenceChunks(source)

		if sourceRemainder == "" && outputRemainder != "" {
			outputChunks = append(outputChunks, outputRemainder)
			outputRemainder = ""
		}

		if len(outputChunks) > 0 {
			p.committedSrc = append(p.committedSrc, sourceChunks...)
			// Cap committedSrc to prevent unbounded growth during long sessions.
			// Anchor matching only needs recent context; older entries cannot
			// realistically overlap with the current Live Captions buffer.
			if len(p.committedSrc) > maxCommittedSrcEntries {
				p.committedSrc = append([]string(nil), p.committedSrc[len(p.committedSrc)-maxCommittedSrcEntries:]...)
			}
		}

		chunks = outputChunks
		remainder = outputRemainder
	}

	if failed && source == p.lastInput && p.retryCount < p.config.MaxRetriesPerSnapshot && !p.retryPending {
		p.retryCount++
		p.retryPending = true
		retrySource = source
		retryDelay = p.config.RetryDelay
	}

	p.translating = false

	if p.queued != "" {
		p.resetDebounceLocked()
	}
	p.mu.Unlock()

	return shouldOutput, retrySource, retryDelay, chunks, remainder
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
