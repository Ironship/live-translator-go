//go:build windows

package text

import "strings"

func NormalizeCaption(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	return strings.Join(strings.Fields(trimmed), " ")
}

func NormalizeCaptionSnapshot(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	normalized := strings.ReplaceAll(trimmed, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.Join(strings.Fields(strings.TrimSpace(rawLine)), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}
