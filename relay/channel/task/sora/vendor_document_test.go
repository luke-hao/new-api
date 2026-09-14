package sora

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVendorDocumentPublicVideoRequests(t *testing.T) {
	for _, tc := range []struct {
		name, model, fields, mode string
	}{
		{"text", "【稳定】sd2.0-720满血（按秒）", `"size":{"value":"1280x720"},"extra":{"resolution":{"value":"720p"},"aspect_ratio":{"value":"16:9"},"watermark":false,"seed":0}`, "text"},
		{"first-url", "happyhorse-1.1-i2v-720p", `"input_reference":{"url":"https://cdn.example/first.jpg"}`, "first_frame"},
		{"first-file-id", "happyhorse-1.1-i2v-720p", `"input_reference":{"file_id":"file_vendor_fixture"}`, "first_frame"},
		{"first-role", "【稳定】sd2.0-720满血（按秒）", `"extra":{"reference_images":[{"url":"https://cdn.example/a.jpg","role":"first_frame"}]}`, "first_frame"},
		{"first-last", "【稳定】sd2.0-720满血（按秒）", `"extra":{"reference_images":[{"image_url":{"url":"https://cdn.example/a.jpg"},"role":"first_frame"},{"url":"https://cdn.example/b.jpg","role":"last_frame"}]}`, "first_last"},
		{"reference", "happyhorse-1.1-r2v-720p", `"extra":{"reference_images":[{"image_url":{"url":"https://cdn.example/a.jpg"},"role":"reference_image"}]}`, "reference"},
		{"all-media", "【稳定】sd2.0-720满血（按秒）", `"extra":{"reference_images":[{"url":"https://cdn.example/a.jpg","role":"reference_image"}],"reference_videos":[{"video_url":"https://cdn.example/motion.mp4"}],"reference_audios":[{"audio_url":{"url":"https://cdn.example/voice.wav"}}]}`, "reference"},
		{"video-edit", "omni-fast-视频编辑（带水印）", `"videos":[{"video_url":"https://cdn.example/motion.mp4"}]`, "video_edit"},
		{"portrait-pixels", "sd2.0-480fast-ad渠道9x16", `"size":"480x854","extra":{"resolution":"480p","aspect_ratio":"9:16"}`, "text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seconds := "10"
			if tc.name == "portrait-pixels" {
				seconds = "15"
			}
			payload := `{"model":"` + tc.model + `","prompt":"A still life with natural light","seconds":` + seconds + `,` + tc.fields + `}`
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			defer common.CleanupBodyStorage(c)
			info := newPlaygroundSoraInfo()
			info.ChannelType = constant.ChannelTypeOpenAI
			info.ChannelBaseUrl = "https://api.aicopy.top"
			info.OriginModelName = tc.model
			info.UpstreamModelName = tc.model
			a := &TaskAdaptor{}
			a.Init(info)
			require.Nil(t, a.ValidateRequestAndSetAction(c, info))
			parsed, err := relaycommon.GetTaskRequest(c)
			require.NoError(t, err)
			require.Equal(t, tc.mode, parsed.Mode)
			body, err := a.BuildRequestBody(c, info)
			require.NoError(t, err)
			raw, err := io.ReadAll(body)
			require.NoError(t, err)
			var sent map[string]any
			require.NoError(t, common.Unmarshal(raw, &sent))
			require.Equal(t, tc.model, sent["model"])
			require.Equal(t, seconds, sent["seconds"])
			require.NotContains(t, sent, "mode")
			require.NotContains(t, string(raw), "generate_audio")
			switch tc.name {
			case "text":
				require.Equal(t, "1280x720", sent["size"])
				require.Equal(t, false, sent["extra"].(map[string]any)["watermark"])
				require.Equal(t, float64(0), sent["extra"].(map[string]any)["seed"])
			case "first-file-id":
				require.Equal(t, map[string]any{"file_id": "file_vendor_fixture"}, sent["input_reference"])
			case "all-media":
				extra := sent["extra"].(map[string]any)
				require.Equal(t, "https://cdn.example/motion.mp4", extra["reference_videos"].([]any)[0].(map[string]any)["url"])
				require.Equal(t, "https://cdn.example/voice.wav", extra["reference_audios"].([]any)[0].(map[string]any)["url"])
			case "first-last":
				require.NotContains(t, sent, "input_reference")
				refs := sent["extra"].(map[string]any)["reference_images"].([]any)
				require.Equal(t, "https://cdn.example/a.jpg", refs[0].(map[string]any)["url"])
				require.Equal(t, "last_frame", refs[1].(map[string]any)["role"])
			case "portrait-pixels":
				require.Equal(t, "480x854", sent["size"])
				require.Equal(t, "9:16", sent["extra"].(map[string]any)["aspect_ratio"])
			}
		})
	}
}

func TestVendorDocumentInvalidRolesFailBeforeSubmission(t *testing.T) {
	for _, refs := range []string{
		`[{"url":"https://cdn.example/a.jpg","role":"last_frame"}]`,
		`[{"url":"https://cdn.example/a.jpg","role":"first_frame"},{"url":"https://cdn.example/b.jpg","role":"reference_image"}]`,
		`[{"url":"https://cdn.example/a.jpg","role":"first_frame"},{"url":"https://cdn.example/b.jpg","role":"first_frame"}]`,
		`[{"url":"https://cdn.example/a.jpg","role":"unknown"}]`,
		`[{"url":"https://cdn.example/a.jpg","role":7}]`,
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{"model":"【稳定】sd2.0-720满血（按秒）","prompt":"test","seconds":10,"extra":{"reference_images":`+refs+`}}`))
		c.Request.Header.Set("Content-Type", "application/json")
		defer common.CleanupBodyStorage(c)
		info := newPlaygroundSoraInfo()
		info.ChannelType = constant.ChannelTypeOpenAI
		info.ChannelBaseUrl = "https://api.aicopy.top"
		e := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
		require.NotNil(t, e)
		require.True(t, e.LocalError)
		require.Equal(t, http.StatusBadRequest, e.StatusCode)
		require.Equal(t, "invalid_reference", e.Code)
	}
}
