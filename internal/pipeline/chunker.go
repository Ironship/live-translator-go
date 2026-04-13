//go:build windows

package pipeline

import (
	"strings"
	"unicode"
)

func consumeSentenceChunks(value string) ([]string, string) {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return nil, ""
	}

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

// chunksDelta returns the elements of incoming that are not already represented
// at the tail of committed. It finds the longest suffix of committed that
// equals a prefix of incoming (by exact string match) and returns the
// non-overlapping tail of incoming as genuinely new chunks.
func chunksDelta(committed []string, incoming []string) []string {
	if len(incoming) == 0 {
		return nil
	}
	if len(committed) == 0 {
		return append([]string(nil), incoming...)
	}

	maxOverlap := len(committed)
	if len(incoming) < maxOverlap {
		maxOverlap = len(incoming)
	}

	for overlap := maxOverlap; overlap > 0; overlap-- {
		if committedSuffixMatchesIncomingPrefix(committed, incoming, overlap) {
			if overlap >= len(incoming) {
				return nil
			}
			return append([]string(nil), incoming[overlap:]...)
		}
	}

	return append([]string(nil), incoming...)
}

func committedSuffixMatchesIncomingPrefix(committed []string, incoming []string, n int) bool {
	if n > len(committed) || n > len(incoming) {
		return false
	}
	for i := 0; i < n; i++ {
		if committed[len(committed)-n+i] != incoming[i] {
			return false
		}
	}
	return true
}

func isSentenceTerminal(value rune) bool {
	switch value {
	case '.', '!', '?', '\u2026':
		return true
	default:
		return false
	}
}

func isSentenceTrailingRune(value rune) bool {
	switch value {
	case '"', '\'', ')', ']', '}', '\u201d', '\u2019':
		return true
	default:
		return false
	}
}
