//go:build windows

package pipeline

import (
	"strings"
	"unicode"

	textutil "live-translator-go/internal/text"
)

const minReliableChunkOverlap = 2
const captionShortThreshold = 10

// extractCurrentCaption returns the last sentence window from a normalized
// multi-line caption snapshot.
func extractCurrentCaption(fullText string) string {
	text := strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(fullText, "\n", " ")), " "))
	if text == "" {
		return ""
	}

	runes := []rune(text)
	effectiveEnd := len(runes)
	for effectiveEnd > 0 && isSentenceTrailingRune(runes[effectiveEnd-1]) {
		effectiveEnd--
	}

	searchUpTo := len(runes)
	if effectiveEnd > 0 && isSentenceTerminal(runes[effectiveEnd-1]) {
		searchUpTo = effectiveEnd - 1
	}

	lastEOS := -1
	for i := searchUpTo - 1; i >= 0; i-- {
		if isSentenceTerminal(runes[i]) {
			lastEOS = i
			break
		}
	}

	latest := strings.TrimSpace(string(runes[lastEOS+1:]))
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
		return true
	}
	return false
}

func mergePendingSource(pending string, current string) (string, bool) {
	pending = textutil.NormalizeCaption(pending)
	current = textutil.NormalizeCaption(current)
	if current == "" {
		return "", false
	}
	if pending == "" {
		return current, false
	}
	if current == pending || strings.HasPrefix(current, pending) {
		return current, false
	}

	pendingTokens := strings.Fields(pending)
	currentTokens := strings.Fields(current)
	_, currentStart, overlap := findLongestSharedRun(pendingTokens, currentTokens)
	if overlap < minOverlap(len(pendingTokens), len(currentTokens)) {
		return current, true
	}

	return strings.Join(currentTokens[currentStart:], " "), false
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

	currentStart, overlap := findAnchorSuffix(anchorTokens, currentTokens, true)
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

func splitForcedChunk(value string, maxWords int, maxChars int, anchorWords int) (string, string) {
	value = textutil.NormalizeCaption(value)
	if value == "" {
		return "", ""
	}

	words := strings.Fields(value)
	if len(words) == 0 {
		return "", ""
	}
	if len(words) < maxWords && len(value) < maxChars {
		return "", value
	}
	if anchorWords < 1 {
		anchorWords = 1
	}

	minChunkWords := anchorWords + 2
	if len(words) <= minChunkWords {
		return "", value
	}

	splitAt := len(words) - anchorWords
	if splitAt < 2 {
		return "", value
	}

	chunk := strings.Join(words[:splitAt], " ")
	remainder := strings.Join(words[splitAt:], " ")
	return textutil.NormalizeCaption(chunk), textutil.NormalizeCaption(remainder)
}

func findLongestSharedRun(left []string, right []string) (int, int, int) {
	bestLeftStart := -1
	bestRightStart := -1
	bestLen := 0

	for leftStart := 0; leftStart < len(left); leftStart++ {
		if len(left)-leftStart < bestLen {
			break
		}

		for rightStart := 0; rightStart < len(right); rightStart++ {
			matchLen := 0
			for leftStart+matchLen < len(left) && rightStart+matchLen < len(right) && left[leftStart+matchLen] == right[rightStart+matchLen] {
				matchLen++
			}

			if matchLen > bestLen || (matchLen == bestLen && matchLen > 0 && rightStart > bestRightStart) {
				bestLeftStart = leftStart
				bestRightStart = rightStart
				bestLen = matchLen
			}
		}
	}

	return bestLeftStart, bestRightStart, bestLen
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
