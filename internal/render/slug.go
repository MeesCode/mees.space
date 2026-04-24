package render

import (
	"strconv"
	"strings"
	"unicode"
)

// Slugify returns a URL-safe slug for the given heading text. Its behaviour
// tracks github-slugger (what rehype-slug uses on the client) closely enough
// for ASCII + common Latin-accented inputs; divergence is caught by the
// frontend parity build script (see scripts/verify-slug-parity.mjs).
//
// seen is modified: every returned slug is recorded so subsequent calls
// against the same map produce -1, -2, ... suffixes for duplicate inputs.
// The first occurrence is bare; the second becomes "foo-1", the third
// "foo-2", etc.
func Slugify(text string, seen map[string]int) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	base := b.String()

	if _, ok := seen[base]; !ok {
		seen[base] = 1
		return base
	}
	// Loop until we find a suffix that has not been used yet, matching
	// github-slugger's while-loop behaviour.
	for {
		candidate := base + "-" + strconv.Itoa(seen[base])
		seen[base]++
		if _, taken := seen[candidate]; !taken {
			seen[candidate] = 1
			return candidate
		}
	}
}
