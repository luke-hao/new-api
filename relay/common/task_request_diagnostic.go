package common

import (
	"regexp"
	"strconv"
)

var videoDiagnosticSelection = regexp.MustCompile(`^[a-zA-Z0-9_:.*-]{1,32}$`)

// TaskRequestDiagnostic excludes prompts, URLs, filenames and file contents.
func TaskRequestDiagnostic(req TaskSubmitReq) map[string]any {
	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	result := map[string]any{"seconds": seconds,
		"images": len(req.Images), "videos": len(req.Videos), "audios": len(req.Audios)}
	values := map[string]string{"mode": req.Mode, "size": req.Size}
	for _, key := range []string{"aspect_ratio", "resolution"} {
		if value, ok := req.Metadata[key].(string); ok {
			values[key] = value
		}
	}
	for key, value := range values {
		if value == "" || videoDiagnosticSelection.MatchString(value) {
			result[key] = value
		}
	}
	return result
}
