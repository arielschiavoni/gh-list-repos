package utils

import "strings"

// alignStrings aligns two strings with maximum padding between them,
// up to a specified maxWidth. If the combined length of the strings
// exceeds maxWidth, they are simply concatenated without padding.
func AlignStrings(s1, s2 string, maxWidth int) string {
	totalLen := len(s1) + len(s2)

	if totalLen > maxWidth {
		// Not enough space for padding, return concatenated strings
		return s1 + s2
	}

	paddingNeeded := maxWidth - totalLen
	padding := strings.Repeat(" ", paddingNeeded)

	return s1 + padding + s2
}
