//go:build windows

package pipeline

import (
	"strings"
)

// captionShortThreshold matches the reference project's TextUtil.SHORT_THRESHOLD (10 bytes).
// When the extracted current-sentence window is shorter than this, we extend it
// back one sentence to provide the translator with better context.
const captionShortThreshold = 10

// extractCurrentCaption returns the last sentence window from a normalised
// multi-line caption snapshot. The algorithm mirrors the reference project
// (SakiRinn/LiveCaptions-Translator) SyncLoop extraction logic:
//
//   - If the snapshot ends with an EOS terminal, we find the second-to-last EOS
//     so the returned window is the last complete sentence.
//   - Otherwise we find the last EOS and return the partial sentence after it.
//   - If the resulting window is shorter than captionShortThreshold bytes, we
//     extend back one more sentence for translation context.
func extractCurrentCaption(fullText string) string {
	// Flatten any newlines to spaces so EOS detection works across lines.
	text := strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(fullText, "\n", " ")), " "))
	if text == "" {
		return ""
	}

	runes := []rune(text)

	// If the last character is an EOS terminal, exclude it from the search so
	// we find the boundary before the last sentence (giving us the full last
	// complete sentence including its terminal).
	searchUpTo := len(runes)
	if isSentenceTerminal(runes[len(runes)-1]) {
		searchUpTo = len(runes) - 1
	}

	lastEOS := -1
	for i := searchUpTo - 1; i >= 0; i-- {
		if isSentenceTerminal(runes[i]) {
			lastEOS = i
			break
		}
	}

	latest := strings.TrimSpace(string(runes[lastEOS+1:]))

	// Extend back one sentence when the window is too short.
	if len(latest) < captionShortThreshold && lastEOS > 0 {
		prevEOS := -1
		for i := lastEOS - 1; i >= 0; i-- {
			if isSentenceTerminal(runes[i]) {
				prevEOS = i
				break
			}
		}
		extended := strings.TrimSpace(string(runes[prevEOS+1:]))
		if extended != "" {
			latest = extended
		}
	}

	return latest
}

// isCompleteCaption reports whether text ends with a sentence-terminal rune,
// meaning it is a finished sentence rather than in-progress speech.
func isCompleteCaption(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	return len(runes) > 0 && isSentenceTerminal(runes[len(runes)-1])
}

func isSentenceTerminal(value rune) bool {
	switch value {
	case '.', '!', '?', '…':
		return true
	default:
		return false
	}
}
