package kling

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newPlaygroundVideoRequest(t *testing.T, references int) *http.Request {
	t.Helper()
	if references == 0 {
		body := `{"model":"kling-v2-master","group":"video","prompt":"ocean sunrise","duration":5,"seconds":"5","size":"1280x720"}`
		request := httptest.NewRequest(http.MethodPost, "/pg/videos", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		return request
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "kling-v2-master"))
	require.NoError(t, writer.WriteField("group", "video"))
	require.NoError(t, writer.WriteField("prompt", "ocean sunrise"))
	require.NoError(t, writer.WriteField("seconds", "5"))
	require.NoError(t, writer.WriteField("size", "1280x720"))
	for i := 0; i < references; i++ {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="input_reference"; filename="frame.png"`)
		header.Set("Content-Type", "image/png")
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', byte(i)})
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/pg/videos", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestPlaygroundVideoMockUpstreamSubmissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	tests := []struct {
		name          string
		references    int
		wantPath      string
		wantTailImage bool
	}{
		{name: "text to video JSON", wantPath: "/v1/videos/text2video"},
		{name: "single image multipart", references: 1, wantPath: "/v1/videos/image2video"},
		{name: "first and last frame multipart", references: 2, wantPath: "/v1/videos/image2video", wantTailImage: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var received requestPayload
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				require.Equal(t, test.wantPath, request.URL.Path)
				require.NoError(t, json.NewDecoder(request.Body).Decode(&received))
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"code":0,"data":{"task_id":"upstream-task"}}`)
			}))
			defer upstream.Close()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = newPlaygroundVideoRequest(t, test.references)
			info := &relaycommon.RelayInfo{
				OriginModelName: "kling-v2-master",
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeKling,
					ChannelBaseUrl:    upstream.URL,
					ApiKey:            "access|secret",
					UpstreamModelName: "kling-v2-master",
				},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{
					PublicTaskID: "task_public",
				},
			}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
			requestBody, err := adaptor.BuildRequestBody(ctx, info)
			require.NoError(t, err)
			response, err := adaptor.DoRequest(ctx, info, requestBody)
			require.NoError(t, err)
			upstreamID, _, taskErr := adaptor.DoResponse(ctx, response, info)
			require.Nil(t, taskErr)
			require.Equal(t, "upstream-task", upstreamID)
			require.Equal(t, "ocean sunrise", received.Prompt)
			require.Equal(t, "kling-v2-master", received.ModelName)
			if test.references == 0 {
				require.Empty(t, received.Image)
			} else {
				require.True(t, strings.HasPrefix(received.Image, "data:image/png;base64,"))
			}
			if test.wantTailImage {
				require.True(t, strings.HasPrefix(received.ImageTail, "data:image/png;base64,"))
			} else {
				require.Empty(t, received.ImageTail)
			}
		})
	}
}

func TestPlaygroundVideoMockUpstreamPolling(t *testing.T) {
	service.InitHttpClient()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/success"):
			_, _ = io.WriteString(w, `{"code":0,"data":{"task_id":"success","task_status":"succeed","task_result":{"videos":[{"url":"https://media.example/video.mp4"}]}}}`)
		case strings.HasSuffix(request.URL.Path, "/failure"):
			_, _ = io.WriteString(w, `{"code":0,"data":{"task_id":"failure","task_status":"failed","task_status_msg":"mock failure"}}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer upstream.Close()

	adaptor := &TaskAdaptor{}
	for _, test := range []struct {
		taskID     string
		wantStatus model.TaskStatus
		wantURL    string
		wantReason string
	}{
		{taskID: "success", wantStatus: model.TaskStatusSuccess, wantURL: "https://media.example/video.mp4"},
		{taskID: "failure", wantStatus: model.TaskStatusFailure, wantReason: "mock failure"},
	} {
		response, err := adaptor.FetchTask(upstream.URL, "access|secret", map[string]any{
			"task_id": test.taskID,
			"action":  constant.TaskActionGenerate,
		}, "")
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		result, err := adaptor.ParseTaskResult(body)
		require.NoError(t, err)
		require.Equal(t, string(test.wantStatus), result.Status)
		require.Equal(t, test.wantURL, result.Url)
		require.Equal(t, test.wantReason, result.Reason)
	}
}

func TestPlaygroundVideoMockUpstreamPollingTimeout(t *testing.T) {
	previousTimeout := common.RelayTimeout
	common.RelayTimeout = 1
	service.InitHttpClient()
	t.Cleanup(func() {
		common.RelayTimeout = previousTimeout
		service.InitHttpClient()
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	adaptor := &TaskAdaptor{}
	_, err := adaptor.FetchTask(upstream.URL, "access|secret", map[string]any{
		"task_id": "timeout",
		"action":  constant.TaskActionGenerate,
	}, "")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "timeout")
}
