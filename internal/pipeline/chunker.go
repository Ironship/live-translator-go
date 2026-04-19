//go:build windows

package pipeline

import (
	"strings"
	"unicode"

	textutil "live-translator-go/internal/text"
)

const (
	minReliableChunkOverlap = 2

	// maxCommittedSrcEntries caps the size of the committedSrc anchor buffer to
	// prevent unbounded growth during long sessions. 32 recent sentences provide
	// enough context for anchor matching against Live Captions' rolling buffer
	// while keeping strings.Join bounded.
	maxCommittedSrcEntries = 32
)

// commonAbbreviations is a small, pragmatic blacklist of words that end with '.'
// but should NOT be treated as sentence terminators when splitting caption text.
// Keys are lowercased and stripped of the trailing period.
var commonAbbreviations = map[string]struct{}{
	// Titles
	"mr": {}, "mrs": {}, "ms": {}, "dr": {}, "st": {},
	"jr": {}, "sr": {}, "prof": {}, "rev": {}, "hon": {},
	"fr": {}, "pr": {}, "gen": {}, "capt": {}, "lt": {}, "sgt": {},
	// Common scholarly / list abbreviations
	"vs": {}, "etc": {}, "cf": {}, "fig": {}, "no": {}, "vol": {}, "pp": {},
	"ch": {}, "ed": {}, "eds": {}, "ca": {},
	// Dotted abbreviations (checked with interior dots intact)
	"e.g": {}, "i.e": {}, "a.m": {}, "p.m": {},
	"u.s": {}, "u.k": {}, "u.s.a": {},
}

func isCompleteCaption(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return false
	}
	i := len(runes) - 1
	for i >= 0 && isSentenceTrailingRune(runes[i]) {
		i--
	}
	if i >= 0 && isSentenceTerminal(runes[i]) {
		// Guard against false positives like "…said Mr." at the end of a partial.
		if runes[i] == '.' && endsWithAbbreviation(string(runes[:i+1])) {
			return false
		}
		return true
	}
	return false
}

// endsWithAbbreviation reports whether the given slice (ending with a '.')
// finishes with a token listed in commonAbbreviations. It is tolerant of
// surrounding punctuation (quotes, parentheses).
func endsWithAbbreviation(text string) bool {
	// Strip the trailing period(s) and any sentence-trailing runes.
	runes := []rune(text)
	end := len(runes)
	for end > 0 && (runes[end-1] == '.' || isSentenceTrailingRune(runes[end-1])) {
		end--
	}
	if end == 0 {
		return false
	}

	// Walk back to the start of the last token (letters, digits, or interior '.').
	start := end
	for start > 0 {
		r := runes[start-1]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' {
			start--
			continue
		}
		break
	}
	token := strings.ToLower(string(runes[start:end]))
	if token == "" {
		return false
	}
	_, ok := commonAbbreviations[token]
	return ok
}

func pendingFromCurrentAfterAnchor(anchor string, current string) string {
	current = textutil.NormalizeCaption(current)
	if current == "" {
		return ""
	}

	anchorTokens := strings.Fields(textutil.NormalizeCaption(anchor))
	currentTokens := strings.Fields(current)
	if len(anchorTokens) == 0 || len(currentTokens) == 0 {
		return current
	}

	anchorCanonical := canonicalizeTokens(anchorTokens)
	currentCanonical := canonicalizeTokens(currentTokens)

	currentStart, overlap := findAnchorSuffix(anchorCanonical, currentCanonical, true)
	if overlap < minOverlap(len(anchorTokens), len(currentTokens)) {
		return current
	}

	nextStart := currentStart + overlap
	if nextStart >= len(currentTokens) {
		return ""
	}

	return strings.Join(currentTokens[nextStart:], " ")
}

func consumeSentenceChunks(value string) ([]string, string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, ""
	}
	if !containsTerminatorChar(trimmed) {
		return nil, trimmed
	}

	runes := []rune(trimmed)

	chunks := make([]string, 0, 2)
	start := 0
	for index := 0; index < len(runes); index++ {
		if !isSentenceTerminal(runes[index]) {
			continue
		}

		end := index + 1
		for end < len(runes) && isSentenceTrailingRune(runes[end]) {
			end++
		}
		if end < len(runes) && !unicode.IsSpace(runes[end]) {
			continue
		}

		// Skip split if the token ending here is a known abbreviation.
		// This prevents "Mr. Smith", "Dr. Jones", "e.g. this" from being
		// mis-split into separate sentences which would corrupt committedSrc.
		if runes[index] == '.' && endsWithAbbreviation(string(runes[start:end])) {
			continue
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		for end < len(runes) && unicode.IsSpace(runes[end]) {
			end++
		}
		start = end
		index = end - 1
	}

	return chunks, strings.TrimSpace(string(runes[start:]))
}

func containsTerminatorChar(value string) bool {
	return strings.ContainsAny(value, ".!?…")
}

func findAnchorSuffix(anchor []string, current []string, preferEarliest bool) (int, int) {
	bestStart := -1
	bestLen := 0

	for length := min(len(anchor), len(current)); length > 0; length-- {
		suffix := anchor[len(anchor)-length:]
		for currentStart := 0; currentStart+length <= len(current); currentStart++ {
			if !tokenSlicesEqual(current[currentStart:currentStart+length], suffix) {
				continue
			}

			if bestStart == -1 || length > bestLen || (length == bestLen && ((preferEarliest && currentStart < bestStart) || (!preferEarliest && currentStart > bestStart))) {
				bestStart = currentStart
				bestLen = length
			}
		}
		if bestLen == length && bestLen > 0 {
			break
		}
	}

	return bestStart, bestLen
}

func tokenSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

// canonicalizeTokens returns a copy of tokens in a form suitable for
// case/punctuation-insensitive equality: lowercased and stripped of trailing
// punctuation that Live Captions commonly inserts or drops between polls
// (commas, colons, semicolons, dashes).
//
// This prevents anchor-matching against committedSrc from failing just because
// a word was capitalized at a sentence boundary in one snapshot and mid-sentence
// in the next, or because a trailing comma appeared/disappeared — failures
// that previously caused the same sentence to be translated twice.
func canonicalizeTokens(tokens []string) []string {
	result := make([]string, len(tokens))
	for i, token := range tokens {
		result[i] = canonicalizeToken(token)
	}
	return result
}

func canonicalizeToken(token string) string {
	trimmed := strings.TrimRight(token, ",:;—–-")
	return strings.ToLower(trimmed)
}

func minOverlap(leftLen int, rightLen int) int {
	shorter := min(leftLen, rightLen)
	if shorter <= 1 {
		return shorter
	}
	if shorter < minReliableChunkOverlap {
		return shorter
	}
	return minReliableChunkOverlap
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func isSentenceTerminal(value rune) bool {
	switch value {
	case '.', '!', '?', '…':
		return true
	default:
		return false
	}
}

func isSentenceTrailingRune(value rune) bool {
	switch value {
	case '"', '\'', ')', ']', '}', '”', '’':
		return true
	default:
		return false
	}
}
