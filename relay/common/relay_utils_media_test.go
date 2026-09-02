package common

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidatePlaygroundVideoMediaModes(t *testing.T) {
	require.Nil(t, validatePlaygroundVideoMedia(&TaskSubmitReq{Mode: "text"}))
	require.NotNil(t, validatePlaygroundVideoMedia(&TaskSubmitReq{Mode: "first_frame"}))
	require.Nil(t, validatePlaygroundVideoMedia(&TaskSubmitReq{
		Mode:   "image",
		Images: []string{"data:image/png;base64,AA=="},
	}))
	require.Nil(t, validatePlaygroundVideoMedia(&TaskSubmitReq{
		Mode:   "first_last",
		Images: []string{"first", "last"},
	}))
	require.NotNil(t, validatePlaygroundVideoMedia(&TaskSubmitReq{
		Mode: "reference",
	}))
	require.Nil(t, validatePlaygroundVideoMedia(&TaskSubmitReq{
		Mode:   "video_edit",
		Videos: []string{"data:video/mp4;base64,AA=="},
	}))
}

func TestValidateMultipartTaskRequestSeparatesReferenceMedia(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "video-model"))
	require.NoError(t, writer.WriteField("group", "default"))
	require.NoError(t, writer.WriteField("prompt", "test prompt"))
	require.NoError(t, writer.WriteField("mode", "reference"))
	require.NoError(t, writer.WriteField("seconds", "6"))
	require.NoError(t, writer.WriteField("extra", `{"aspect_ratio":"16:9","resolution":"720p"}`))

	for _, item := range []struct {
		field       string
		filename    string
		contentType string
		data        []byte
	}{
		{field: "reference_images", filename: "image.png", contentType: "image/png", data: []byte("image")},
		{field: "reference_videos", filename: "video.mp4", contentType: "video/mp4", data: []byte("video")},
		{field: "reference_audios", filename: "audio.wav", contentType: "audio/wav", data: []byte("audio")},
	} {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="`+item.field+`"; filename="`+item.filename+`"`)
		header.Set("Content-Type", item.contentType)
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write(item.data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	req, err := validateMultipartTaskRequest(ctx, &RelayInfo{}, constant.TaskActionGenerate)
	require.NoError(t, err)
	require.Equal(t, 6, req.Duration)
	require.Len(t, req.Images, 1)
	require.Len(t, req.Videos, 1)
	require.Len(t, req.Audios, 1)
	require.Equal(t, "16:9", req.Metadata["aspect_ratio"])
	require.Equal(t, "720p", req.Metadata["resolution"])
}

func TestValidateMultipartTaskRequestPreservesFirstLastAndExtra(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "video-model"))
	require.NoError(t, writer.WriteField("prompt", "test prompt"))
	require.NoError(t, writer.WriteField("mode", "first_last"))
	require.NoError(t, writer.WriteField("extra", `{"aspect_ratio":"16:9","resolution":"720p"}`))
	for _, name := range []string{"first.png", "last.png"} {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="input_reference"; filename="`+name+`"`)
		header.Set("Content-Type", "image/png")
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write([]byte("image"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	req, err := validateMultipartTaskRequest(ctx, &RelayInfo{}, constant.TaskActionGenerate)
	require.NoError(t, err)
	require.Len(t, req.Images, 2)
	require.Equal(t, req.Images[0], req.InputReference)
	require.Equal(t, "16:9", req.Metadata["aspect_ratio"])
	require.Equal(t, "720p", req.Metadata["resolution"])
}

func TestValidatePlaygroundVideoDataReferenceTypeAndSize(t *testing.T) {
	require.Error(t, validatePlaygroundDataReference("data:video/mp4;base64,AA==", "image/", 1024))
	require.NoError(t, validatePlaygroundDataReference("https://cdn.example/video.mp4", "video/", 1))
	require.Error(t, validatePlaygroundDataReference("data:image/png;base64,%%%", "image/", 1024))
}

func TestValidatePlaygroundVideoParametersUsesMappedModelProfile(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("model_mapping", `{"studio-video":"official-h3-1080p"}`)
	info := &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeMiniMax}}
	req := &TaskSubmitReq{
		Model:    "studio-video",
		Mode:     "reference",
		Duration: 5,
		Images:   []string{"https://cdn.example/reference.jpg"},
		Metadata: map[string]interface{}{
			"aspect_ratio": "16:9",
			"resolution":   "1080p",
		},
	}
	require.Nil(t, validatePlaygroundVideoParameters(ctx, info, req))

	req.Duration = 4
	require.NotNil(t, validatePlaygroundVideoParameters(ctx, info, req))
	req.Duration = 5
	req.Videos = []string{"https://cdn.example/reference.mp4"}
	require.NotNil(t, validatePlaygroundVideoParameters(ctx, info, req))
}
