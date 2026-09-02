/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package common

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// PlaygroundVideoCapability is the shared contract used by the capability
// endpoint and the video relay validator. Empty ratio/resolution lists mean
// that the provider does not expose that control.
type PlaygroundVideoCapability struct {
	Modes              []string
	Durations          []int
	AspectRatios       []string
	Resolutions        []string
	SupportsSeed       bool
	MaxInputReferences int
	MaxImageReferences int
	MaxVideoReferences int
	MaxAudioReferences int
	MaxImageBytes      int64
	MaxVideoBytes      int64
	MaxAudioBytes      int64
	MaxVideoEditBytes  int64
}

const (
	PlaygroundVideoModeText      = "text"
	PlaygroundVideoModeFirst     = "first_frame"
	PlaygroundVideoModeFirstLast = "first_last"
	PlaygroundVideoModeReference = "reference"
	PlaygroundVideoModeEdit      = "video_edit"
)

func playgroundVideoFullModes() []string {
	return []string{
		PlaygroundVideoModeText,
		PlaygroundVideoModeFirst,
		PlaygroundVideoModeFirstLast,
		PlaygroundVideoModeReference,
	}
}

func playgroundVideoRange(start, end int) []int {
	values := make([]int, 0, end-start+1)
	for value := start; value <= end; value++ {
		values = append(values, value)
	}
	return values
}

func playgroundVideoResolution(name string, fallback []string) []string {
	switch {
	case strings.Contains(name, "2k"):
		return []string{"2k"}
	case strings.Contains(name, "1080"):
		return []string{"1080p"}
	case strings.Contains(name, "768"):
		return []string{"768p"}
	case strings.Contains(name, "480"):
		return []string{"480p"}
	case strings.Contains(name, "720"):
		return []string{"720p"}
	default:
		return fallback
	}
}

func setPlaygroundVideoDefaults(capability *PlaygroundVideoCapability) {
	capability.MaxImageBytes = 15 << 20
	capability.MaxVideoBytes = 160 << 20
	capability.MaxAudioBytes = 50 << 20
	capability.MaxVideoEditBytes = 8 << 20
}

// GetPlaygroundVideoCapability returns the effective controls for the model
// routed through a channel. The model name is the mapped upstream name.
func GetPlaygroundVideoCapability(channelType int, modelName string) (PlaygroundVideoCapability, bool) {
	name := strings.ToLower(strings.TrimSpace(modelName))
	capability := PlaygroundVideoCapability{}
	setPlaygroundVideoDefaults(&capability)

	switch channelType {
	case constant.ChannelTypeAli:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast}
		capability.Durations = []int{5, 10}
		capability.AspectRatios = []string{"16:9", "9:16", "1:1"}
		capability.Resolutions = []string{"480p", "720p", "1080p"}
		capability.SupportsSeed = true
		capability.MaxImageReferences = 2
	case constant.ChannelTypeKling:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast}
		capability.Durations = []int{5, 10}
		capability.AspectRatios = []string{"16:9", "9:16", "1:1"}
		capability.MaxImageReferences = 2
	case constant.ChannelTypeJimeng:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast}
		capability.Durations = []int{5, 10}
		capability.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4"}
		capability.SupportsSeed = true
		capability.MaxImageReferences = 2
	case constant.ChannelTypeVidu:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast}
		capability.Durations = []int{4, 5, 8}
		capability.Resolutions = []string{"720p", "1080p"}
		capability.SupportsSeed = true
		capability.MaxImageReferences = 2
	case constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast}
		capability.Durations = []int{5, 10}
		capability.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
		capability.Resolutions = []string{"480p", "720p", "1080p"}
		capability.SupportsSeed = true
		capability.MaxImageReferences = 2
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst}
		capability.Durations = []int{4, 5, 6, 8}
		capability.AspectRatios = []string{"16:9", "9:16"}
		capability.Resolutions = []string{"720p", "1080p"}
		capability.MaxImageReferences = 1
	case constant.ChannelTypeSora, constant.ChannelTypeOpenAI:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst}
		capability.Durations = []int{4, 8, 12}
		capability.AspectRatios = []string{"16:9", "9:16"}
		capability.Resolutions = []string{"720p", "1080p"}
		capability.MaxImageReferences = 1
	case constant.ChannelTypeMiniMax:
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast}
		capability.Durations = []int{6, 10}
		capability.Resolutions = []string{"720p", "768p", "1080p"}
		capability.MaxImageReferences = 2
	default:
		return PlaygroundVideoCapability{}, false
	}

	applyPlaygroundVideoModelProfile(&capability, name)
	if capability.MaxImageReferences == 0 {
		capability.MaxImageReferences = capability.MaxInputReferences
	}
	capability.MaxInputReferences = capability.MaxImageReferences
	return capability, len(capability.Modes) > 0
}

func applyPlaygroundVideoModelProfile(capability *PlaygroundVideoCapability, name string) {
	if capability == nil || name == "" {
		return
	}
	fullModes := playgroundVideoFullModes()

	switch {
	case strings.Contains(name, "happyhorse"), strings.Contains(name, "happy-horse"):
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeReference}
		capability.Durations = playgroundVideoRange(4, 15)
		capability.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "4:5", "5:4", "9:21", "21:9"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"720p"})
		capability.MaxImageReferences = 9
		capability.MaxVideoReferences = 0
		capability.MaxAudioReferences = 0
	case strings.Contains(name, "h3"):
		capability.Modes = []string{PlaygroundVideoModeReference}
		capability.Durations = playgroundVideoRange(5, 15)
		capability.AspectRatios = []string{"1:1", "2:3", "3:2", "3:4", "4:3", "9:16", "16:9", "21:9"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"720p", "1080p", "2k"})
		capability.MaxImageReferences = 9
		capability.MaxVideoReferences = 0
		capability.MaxAudioReferences = 3
	case strings.Contains(name, "omni"):
		capability.Durations = []int{10}
		capability.AspectRatios = []string{"16:9", "9:16"}
		capability.Resolutions = []string{"720p"}
		capability.MaxImageReferences = 5
		capability.MaxVideoReferences = 2
		capability.MaxAudioReferences = 0
		capability.MaxImageBytes = 8 << 20
		if strings.Contains(name, "视频编辑") || strings.Contains(name, "video-edit") || strings.Contains(name, "video_edit") {
			capability.Modes = []string{PlaygroundVideoModeEdit}
		} else {
			capability.Modes = fullModes
		}
	case strings.Contains(name, "grok"):
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst, PlaygroundVideoModeReference}
		capability.Durations = []int{6, 10, 15}
		if strings.Contains(name, "按秒") || strings.Contains(name, "per-second") {
			capability.Durations = playgroundVideoRange(6, 15)
		}
		capability.AspectRatios = []string{"16:9", "9:16", "3:2"}
		capability.Resolutions = []string{"720p"}
		capability.MaxImageReferences = 7
		capability.MaxVideoReferences = 0
		capability.MaxAudioReferences = 0
	case strings.Contains(name, "veo"):
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst}
		capability.Durations = []int{4, 5, 6, 8}
		capability.AspectRatios = []string{"16:9", "9:16"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"720p", "1080p"})
		capability.MaxImageReferences = 1
		capability.MaxVideoReferences = 0
		capability.MaxAudioReferences = 0
	case strings.Contains(name, "sora"):
		capability.Modes = []string{PlaygroundVideoModeText, PlaygroundVideoModeFirst}
		capability.Durations = []int{4, 8, 12}
		capability.AspectRatios = []string{"16:9", "9:16"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"720p", "1080p"})
		capability.MaxImageReferences = 1
		capability.MaxVideoReferences = 0
		capability.MaxAudioReferences = 0
	case strings.Contains(name, "wang-3"), strings.Contains(name, "wang3"), strings.Contains(name, "wan-3"), strings.Contains(name, "wan3"):
		capability.Modes = fullModes
		capability.Durations = playgroundVideoRange(4, 30)
		capability.AspectRatios = []string{"16:9", "4:3", "1:1", "3:4", "9:16"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"480p", "720p"})
		capability.MaxImageReferences = 10
		if strings.Contains(name, "官方") || strings.Contains(name, "official") {
			capability.MaxVideoReferences = 5
			capability.MaxAudioReferences = 5
		} else {
			capability.MaxVideoReferences = 0
			capability.MaxAudioReferences = 0
		}
	case strings.Contains(name, "sd-2.5"), strings.Contains(name, "sd2.5"), strings.Contains(name, "sd25"), strings.Contains(name, "2.5") && (strings.Contains(name, "sd") || strings.Contains(name, "官方") || strings.Contains(name, "official")):
		capability.Modes = fullModes
		capability.Durations = playgroundVideoRange(4, 30)
		capability.AspectRatios = []string{"16:9", "9:16", "1:1"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"480p", "720p"})
		capability.MaxImageReferences = 30
		capability.MaxVideoReferences = 10
		capability.MaxAudioReferences = 10
		if strings.Contains(name, "官方") || strings.Contains(name, "official") || strings.Contains(name, "ark") {
			capability.Modes = []string{PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast, PlaygroundVideoModeReference}
			capability.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
		}
	case strings.Contains(name, "900") || (strings.Contains(name, "特惠") && strings.Contains(name, "sd")):
		capability.Modes = []string{PlaygroundVideoModeReference}
		capability.Durations = []int{15}
		capability.AspectRatios = []string{"16:9", "9:16"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"720p"})
		capability.MaxImageReferences = 9
		capability.MaxVideoReferences = 0
		capability.MaxAudioReferences = 0
	case strings.Contains(name, "sd-720"), strings.Contains(name, "sd2.0"), strings.Contains(name, "sd-2.0"), strings.Contains(name, "sd20"):
		capability.Modes = fullModes
		capability.Durations = playgroundVideoRange(4, 15)
		capability.AspectRatios = []string{"16:9", "9:16", "1:1"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"720p"})
		capability.MaxImageReferences = 9
		capability.MaxVideoReferences = 3
		capability.MaxAudioReferences = 3
		if strings.Contains(name, "933") {
			capability.AspectRatios = []string{"16:9", "9:16"}
		}
		if strings.Contains(name, "官方") || strings.Contains(name, "official") {
			capability.AspectRatios = []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}
		}
		if strings.Contains(name, "ad渠道") || strings.Contains(name, "ad-channel") {
			capability.Durations = []int{15}
			capability.AspectRatios = []string{"16:9", "9:16"}
		}
	case strings.Contains(name, "kling"):
		capability.Durations = []int{5, 10}
		capability.AspectRatios = []string{"16:9", "9:16", "1:1"}
	case strings.Contains(name, "vidu"):
		capability.Durations = []int{4, 5, 8}
		capability.Resolutions = []string{"720p", "1080p"}
	case strings.Contains(name, "jimeng") || strings.Contains(name, "即梦"):
		capability.Durations = []int{5, 10}
		capability.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4"}
	case strings.Contains(name, "minimax") || strings.Contains(name, "hailuo"):
		capability.Durations = []int{6, 10}
		capability.Resolutions = []string{"720p", "768p", "1080p"}
	case strings.Contains(name, "测试模型") || strings.Contains(name, "test-model"):
		capability.Modes = fullModes
		capability.Durations = playgroundVideoRange(4, 30)
		capability.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
		capability.Resolutions = playgroundVideoResolution(name, []string{"480p", "720p", "1080p"})
		capability.MaxImageReferences = 30
		capability.MaxVideoReferences = 10
		capability.MaxAudioReferences = 10
	}

	if strings.Contains(name, "t2v") {
		capability.Modes = []string{PlaygroundVideoModeText}
	} else if strings.Contains(name, "flf2v") || strings.Contains(name, "first-last") || strings.Contains(name, "first_tail") {
		capability.Modes = []string{PlaygroundVideoModeFirstLast}
	} else if strings.Contains(name, "i2v") {
		capability.Modes = []string{PlaygroundVideoModeFirst}
		if strings.Contains(name, "happyhorse") || strings.Contains(name, "happy-horse") {
			capability.AspectRatios = []string{"跟随首帧"}
		}
	} else if strings.Contains(name, "r2v") || strings.Contains(name, "reference") {
		capability.Modes = []string{PlaygroundVideoModeReference}
	}
}
