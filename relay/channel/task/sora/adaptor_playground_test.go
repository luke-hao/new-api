package sora

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newPlaygroundSoraInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeSora,
			ChannelBaseUrl:    "https://upstream.example",
			ApiKey:            "test-key",
			UpstreamModelName: "sora-2",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
}

func TestPlaygroundJSONStripsInternalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"studio-alias","group":"video","prompt":"ocean sunrise","duration":5,"seconds":"5","size":"1280x720","metadata":{"seed":7}}`
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	defer common.CleanupBodyStorage(ctx)

	adaptor := &TaskAdaptor{}
	info := newPlaygroundSoraInfo()
	adaptor.Init(info)
	upstreamBody, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(upstreamBody).Decode(&payload))
	require.Equal(t, "sora-2", payload["model"])
	require.Equal(t, "5", payload["seconds"])
	require.NotContains(t, payload, "group")
	require.NotContains(t, payload, "duration")
	require.NotContains(t, payload, "metadata")
}

func TestPlaygroundMultipartStripsInternalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"model":    "studio-alias",
		"group":    "video",
		"prompt":   "ocean sunrise",
		"duration": "5",
		"seconds":  "5",
		"size":     "1280x720",
		"metadata": `{"seed":7}`,
	} {
		require.NoError(t, writer.WriteField(key, value))
	}
	file, err := writer.CreateFormFile("input_reference", "frame.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("frame-data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	defer common.CleanupBodyStorage(ctx)

	adaptor := &TaskAdaptor{}
	info := newPlaygroundSoraInfo()
	adaptor.Init(info)
	upstreamBody, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)

	_, params, err := mime.ParseMediaType(ctx.Request.Header.Get("Content-Type"))
	require.NoError(t, err)
	form, err := multipart.NewReader(upstreamBody, params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	defer form.RemoveAll()
	require.Equal(t, []string{"sora-2"}, form.Value["model"])
	require.Equal(t, []string{"5"}, form.Value["seconds"])
	require.NotContains(t, form.Value, "group")
	require.NotContains(t, form.Value, "duration")
	require.NotContains(t, form.Value, "metadata")
	require.Len(t, form.File["input_reference"], 1)

	input, err := form.File["input_reference"][0].Open()
	require.NoError(t, err)
	data, err := io.ReadAll(input)
	require.NoError(t, err)
	require.NoError(t, input.Close())
	require.Equal(t, []byte("frame-data"), data)
}
