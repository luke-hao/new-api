package sora

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func aicopyPixelSize(extra map[string]any) string {
	resolution, _ := extra["resolution"].(string)
	edge := map[string]int{"480p": 480, "720p": 720, "768p": 768, "1080p": 1080, "2k": 1440, "4k": 2160}[strings.ToLower(strings.TrimSpace(resolution))]
	if edge == 0 {
		return ""
	}
	ratio, _ := extra["aspect_ratio"].(string)
	if ratio == "" {
		ratio, _ = extra["aspectRatio"].(string)
	}
	if ratio == "" {
		ratio, _ = extra["ratio"].(string)
	}
	pair := strings.Split(strings.TrimSpace(ratio), ":")
	if len(pair) != 2 {
		return ""
	}
	width, err := strconv.Atoi(pair[0])
	if err != nil || width <= 0 {
		return ""
	}
	height, err := strconv.Atoi(pair[1])
	if err != nil || height <= 0 {
		return ""
	}
	// Video dimensions must be even. Resolution denotes the short edge.
	if width >= height {
		return fmt.Sprintf("%dx%d", int(math.Ceil(float64(edge)*float64(width)/float64(height)/2))*2, edge)
	}
	return fmt.Sprintf("%dx%d", edge, int(math.Ceil(float64(edge)*float64(height)/float64(width)/2))*2)
}
