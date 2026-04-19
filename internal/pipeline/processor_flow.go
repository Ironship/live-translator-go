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

	var (
		translated string
		err        error
	)

	if p.config.StreamingEnabled {
		if streamer, ok := p.translator.(StreamingTranslator); ok {
			translated, err = streamer.TranslateStream(requestCtx, source, func(partial string) {
				normalized := textutil.NormalizeCaptionSnapshot(partial)
				if normalized == "" {
					return
				}
				// Emit the incremental translation as a partial chunk so the
				// overlay can render "typing" output without committing it.
				p.output.PushCaption(nil, normalized)
			})
		} else {
			translated, err = p.translator.Translate(requestCtx, source)
		}
	} else {
		translated, err = p.translator.Translate(requestCtx, source)
	}

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

		if p.config.ShowOriginal {
			outputChunks, outputRemainder = interleaveBilingualChunks(sourceChunks, sourceRemainder, outputChunks, outputRemainder)
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

// interleaveBilingualChunks pairs committed source chunks with their matching
// translated chunks so the overlay renders alternating original/translated
// lines. When the two slices are not aligned (more translations than sources,
// or vice versa), extra entries fall through unpaired at the end.
func interleaveBilingualChunks(sourceChunks []string, sourceRemainder string, outputChunks []string, outputRemainder string) ([]string, string) {
	if len(sourceChunks) == 0 {
		return outputChunks, outputRemainder
	}

	paired := make([]string, 0, len(sourceChunks)+len(outputChunks))
	n := len(sourceChunks)
	if len(outputChunks) < n {
		n = len(outputChunks)
	}
	for i := 0; i < n; i++ {
		paired = append(paired, sourceChunks[i])
		paired = append(paired, outputChunks[i])
	}
	paired = append(paired, sourceChunks[n:]...)
	paired = append(paired, outputChunks[n:]...)

	// If there is a trailing translated remainder, prepend the matching source
	// fragment (if any) so the partial line also appears bilingual.
	if outputRemainder != "" && sourceRemainder != "" {
		return paired, sourceRemainder + "\n" + outputRemainder
	}
	return paired, outputRemainder
}
