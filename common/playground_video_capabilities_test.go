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
	"os"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundVideoTargetProfiles(t *testing.T) {
	happyHorse, ok := GetPlaygroundVideoCapability(constant.ChannelTypeDoubaoVideo, "happyhorse-1.1-i2v-1080p")
	require.True(t, ok)
	require.Equal(t, []string{PlaygroundVideoModeFirst}, happyHorse.Modes)
	require.Equal(t, []string{"跟随首帧"}, happyHorse.AspectRatios)
	require.Equal(t, []string{"1080p"}, happyHorse.Resolutions)
	require.Equal(t, playgroundVideoRange(4, 15), happyHorse.Durations)

	officialWang, ok := GetPlaygroundVideoCapability(constant.ChannelTypeDoubaoVideo, "官方wang3.0-720p")
	require.True(t, ok)
	require.Equal(t, 10, officialWang.MaxImageReferences)
	require.Equal(t, 5, officialWang.MaxVideoReferences)
	require.Equal(t, 5, officialWang.MaxAudioReferences)

	officialSD25, ok := GetPlaygroundVideoCapability(constant.ChannelTypeVolcEngine, "【官方稳定版】2.5-480p")
	require.True(t, ok)
	require.Equal(t, []string{PlaygroundVideoModeFirst, PlaygroundVideoModeFirstLast, PlaygroundVideoModeReference}, officialSD25.Modes)
	require.Equal(t, playgroundVideoRange(4, 30), officialSD25.Durations)
	require.Equal(t, []string{"480p"}, officialSD25.Resolutions)
	require.Equal(t, 30, officialSD25.MaxImageReferences)

	specialSD, ok := GetPlaygroundVideoCapability(constant.ChannelTypeDoubaoVideo, "sd2.0-720fast（特惠）")
	require.True(t, ok)
	require.Equal(t, []string{PlaygroundVideoModeReference}, specialSD.Modes)
	require.Equal(t, []int{15}, specialSD.Durations)
	require.Zero(t, specialSD.MaxVideoReferences)

	omniEdit, ok := GetPlaygroundVideoCapability(constant.ChannelTypeDoubaoVideo, "omni-fast-视频编辑（无水印）")
	require.True(t, ok)
	require.Equal(t, []string{PlaygroundVideoModeEdit}, omniEdit.Modes)
	require.Equal(t, 2, omniEdit.MaxVideoReferences)
	require.Equal(t, int64(8<<20), omniEdit.MaxVideoEditBytes)
}

// Captured from the reference generator and the upstream group's published limits.
func TestPlaygroundVideoAICopyCatalog(t *testing.T) {
	data, err := os.ReadFile("testdata/playground_video_aicopy.json")
	require.NoError(t, err)
	var expected []struct {
		Model        string   `json:"model"`
		Modes        []string `json:"modes"`
		Durations    []int    `json:"durations"`
		AspectRatios []string `json:"aspect_ratios"`
		Resolutions  []string `json:"resolutions"`
		Images       int      `json:"max_image_references"`
		Videos       int      `json:"max_video_references"`
		Audios       int      `json:"max_audio_references"`
	}
	require.NoError(t, Unmarshal(data, &expected))
	require.Len(t, expected, 45)
	for _, want := range expected {
		t.Run(want.Model, func(t *testing.T) {
			got, ok := GetPlaygroundVideoCapability(constant.ChannelTypeOpenAI, want.Model)
			require.True(t, ok)
			require.Equal(t, want.Modes, got.Modes)
			require.Equal(t, want.Durations, got.Durations)
			require.Equal(t, want.AspectRatios, got.AspectRatios)
			require.Equal(t, want.Resolutions, got.Resolutions)
			require.Equal(t, want.Images, got.MaxImageReferences)
			require.Equal(t, want.Videos, got.MaxVideoReferences)
			require.Equal(t, want.Audios, got.MaxAudioReferences)
		})
	}
}
