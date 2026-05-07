package ids

import (
	"fmt"
	"strings"
	"unicode"
)

func Slugify(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// DedupeSlug returns slug if absent, else slug-2, slug-3, ...
func DedupeSlug(slug string, existing map[string]bool) string {
	if !existing[slug] {
		return slug
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d", slug, i)
		if !existing[c] {
			return c
		}
	}
}

func MakeSessionID(project, agent, slug string) string {
	return fmt.Sprintf("cleo-%s-%s-%s", project, agent, slug)
}
