package common

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestTaskSubmitReqNormalizesUnifiedVideoFields(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{
		"model":"video-model",
		"prompt":"keep the subject consistent",
		"seconds":8,
		"input_reference":{"url":"https://cdn.example/first.jpg"},
		"extra":{
			"aspect_ratio":"16:9",
			"resolution":"720p",
			"seed":42,
			"reference_images":[
				{"url":"https://cdn.example/first.jpg","role":"first_frame"},
				{"image_url":{"url":"https://cdn.example/last.jpg"},"role":"last_frame"}
			]
		}
	}`), &req)
	require.NoError(t, err)
	require.Equal(t, "8", req.Seconds)
	require.Equal(t, "https://cdn.example/first.jpg", req.InputReference)
	require.Equal(t, []string{
		"https://cdn.example/first.jpg",
		"https://cdn.example/last.jpg",
	}, req.Images)
	require.Equal(t, "16:9", req.Metadata["aspect_ratio"])
	require.Equal(t, "720p", req.Metadata["resolution"])
	require.Equal(t, float64(42), req.Metadata["seed"])
}

func TestTaskSubmitReqNormalizesNumericMetadataStrings(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{"model":"video-model","prompt":"test","extra":{"fps":"24","seed":"42"}}`), &req)
	require.NoError(t, err)
	require.Equal(t, float64(24), req.Metadata["fps"])
	require.Equal(t, float64(42), req.Metadata["seed"])
}

func TestTaskSubmitReqRejectsInvalidExtra(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{"model":"video-model","prompt":"test","extra":"invalid"}`), &req)
	require.Error(t, err)
}

func TestTaskSubmitReqRejectsInvalidReferenceShape(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{"model":"video-model","prompt":"test","input_reference":{"role":"first_frame"}}`), &req)
	require.Error(t, err)
}

func TestTaskSubmitReqNormalizesImageReferenceAliases(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{"model":"video-model","prompt":"test","input_reference":{"image_url":{"url":"https://cdn.example/image.jpg"}}}`), &req)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example/image.jpg", req.InputReference)
	require.Equal(t, []string{"https://cdn.example/image.jpg"}, req.Images)
}

func TestTaskSubmitReqNormalizesReferenceAliasesAndPreservesRoles(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{
		"model":"video-model",
		"prompt":"test",
		"duration":"10",
		"size":{"value":"1280x720"},
		"extra":{
			"aspect_ratio":{"value":"16:9"},
			"fps":"24",
			"reference_images":[
				{"image_url":{"url":"https://cdn.example/first.jpg"},"role":"first_frame"},
				{"url":"https://cdn.example/last.jpg","role":"last_frame"}
			],
			"reference_videos":[{"url":"https://cdn.example/motion.mp4"}],
			"reference_audios":[{"url":"https://cdn.example/voice.wav"}]
		}
	}`), &req)
	require.NoError(t, err)
	require.Equal(t, 10, req.Duration)
	require.Equal(t, "1280x720", req.Size)
	require.Equal(t, "16:9", req.Metadata["aspect_ratio"])
	require.Equal(t, []string{"https://cdn.example/first.jpg", "https://cdn.example/last.jpg"}, req.Images)
	references, ok := req.Metadata["reference_images"].([]any)
	require.True(t, ok)
	require.Equal(t, "first_frame", references[0].(map[string]any)["role"])
}

func TestTaskSubmitReqRejectsEmptyReference(t *testing.T) {
	var req TaskSubmitReq
	err := json.Unmarshal([]byte(`{"model":"video-model","prompt":"test","input_reference":{"url":""}}`), &req)
	require.Error(t, err)
	err = json.Unmarshal([]byte(`{"model":"video-model","prompt":"test","input_reference":123}`), &req)
	require.Error(t, err)
}
