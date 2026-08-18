package common

import (
	"regexp"
	"strings"
)

var requestIDAnnotationPattern = regexp.MustCompile(`(?i)\s*\(request id:\s*[^)]*\)`)

// StripRequestIDAnnotations keeps request IDs in their structured log fields
// instead of repeatedly nesting every proxy hop in the public error message.
func StripRequestIDAnnotations(message string) string {
	return strings.TrimSpace(requestIDAnnotationPattern.ReplaceAllString(message, ""))
}
