package sora

import (
	"bytes"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strconv"
	"testing"
)

func TestAICopyBillingUnitsAllConnectedModels(t *testing.T) {
	data, err := os.ReadFile("../../../../common/testdata/aicopy_pricing_20260913.json")
	require.NoError(t, err)
	var snapshot struct {
		Data []struct {
			Model string  `json:"model_name"`
			Unit  string  `json:"price_unit"`
			Price float64 `json:"model_price"`
		}
	}
	require.NoError(t, common.Unmarshal(data, &snapshot))
	for _, row := range snapshot.Data {
		t.Run(row.Model, func(t *testing.T) {
			for _, duration := range []int{5, 15, 30} {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Set("task_request", relaycommon.TaskSubmitReq{Seconds: strconv.Itoa(duration), Size: "1792x1024"})
				info := newPlaygroundSoraInfo()
				info.OriginModelName = row.Model
				ratios := (&TaskAdaptor{}).EstimateBilling(c, info)
				seconds := 1.
				if row.Unit == "秒" {
					seconds = float64(duration)
				}
				require.Equal(t, seconds, ratios["seconds"])
				require.Equal(t, 1., ratios["size"])
				require.InDelta(t, row.Price*1.35*seconds, row.Price*1.35*ratios["seconds"]*ratios["size"], .0000001)
			}
		})
	}
}
func TestAICopyStudioProtocolUploadsAndRoles(t *testing.T) {
	service.InitHttpClient()
	for _, mode := range []string{"text", "first_frame", "first_last", "reference"} {
		t.Run(mode, func(t *testing.T) {
			uploads, creates, queries := 0, 0, 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
				switch r.URL.Path {
				case "/v1/uploads":
					uploads++
					require.NoError(t, r.ParseMultipartForm(1<<20))
					defer r.MultipartForm.RemoveAll()
					require.Len(t, r.MultipartForm.File, 1)
					for _, files := range r.MultipartForm.File {
						require.Len(t, files, 1)
						require.Equal(t, "image/png", files[0].Header.Get("Content-Type"))
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":{"url":"https://cdn.example/reference.png"}}`)
				case "/v1/videos":
					creates++
					require.Equal(t, "application/json", r.Header.Get("Content-Type"))
					var body map[string]any
					require.NoError(t, common.DecodeJson(r.Body, &body))
					for _, field := range []string{"mode", "group", "duration", "metadata"} {
						require.NotContains(t, body, field)
					}
					require.Equal(t, "1280x720", body["size"])
					extra := body["extra"].(map[string]any)
					require.Equal(t, "720p", extra["resolution"])
					require.Equal(t, "16:9", extra["aspect_ratio"])
					switch mode {
					case "first_frame":
						require.Equal(t, map[string]any{"url": "https://cdn.example/reference.png"}, body["input_reference"])
					case "first_last":
						require.NotContains(t, body, "input_reference")
						refs := extra["reference_images"].([]any)
						require.Len(t, refs, 2)
						require.Equal(t, "first_frame", refs[0].(map[string]any)["role"])
						require.Equal(t, "last_frame", refs[1].(map[string]any)["role"])
					case "reference":
						require.Equal(t, "reference_image", extra["reference_images"].([]any)[0].(map[string]any)["role"])
					}
					_, _ = io.WriteString(w, `{"id":"task_upstream","status":"queued","seconds":"15"}`)
				case "/v1/videos/task_upstream":
					queries++
					_, _ = io.WriteString(w, `{"id":"task_upstream","status":"completed","video_url":"https://cdn.example/video.mp4"}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer upstream.Close()
			var b bytes.Buffer
			ct := "application/json"
			if mode == "text" {
				b.WriteString(`{"model":"【稳定】sd2.0-720满血（按秒）","prompt":"ocean","mode":"text","seconds":15,"size":"720p","extra":{"resolution":"720p","aspect_ratio":"16:9"}}`)
			} else {
				writer := multipart.NewWriter(&b)
				for k, v := range map[string]string{"model": "【稳定】sd2.0-720满血（按秒）", "prompt": "ocean", "mode": mode, "seconds": "15", "size": "720p", "extra": `{"resolution":"720p","aspect_ratio":"16:9"}`} {
					require.NoError(t, writer.WriteField(k, v))
				}
				field := "input_reference"
				count := 1
				if mode == "reference" {
					field = "reference_images"
				}
				if mode == "first_last" {
					count = 2
				}
				for i := 0; i < count; i++ {
					h := make(textproto.MIMEHeader)
					h.Set("Content-Disposition", `form-data; name="`+field+`"; filename="frame.png"`)
					h.Set("Content-Type", "image/png")
					file, err := writer.CreatePart(h)
					require.NoError(t, err)
					_, err = file.Write([]byte("\x89PNG\r\n\x1a\nfixture"))
					require.NoError(t, err)
				}
				require.NoError(t, writer.Close())
				ct = writer.FormDataContentType()
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", &b)
			c.Request.Header.Set("Content-Type", ct)
			defer common.CleanupBodyStorage(c)
			info := newPlaygroundSoraInfo()
			info.ChannelType = constant.ChannelTypeOpenAI
			info.ChannelBaseUrl = upstream.URL
			info.UpstreamModelName = "【稳定】sd2.0-720满血（按秒）"
			info.PublicTaskID = "task_public"
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			body, err := adaptor.buildAICopyVideoBody(c, info)
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, upstream.URL+"/v1/videos", body)
			require.NoError(t, err)
			require.NoError(t, adaptor.BuildRequestHeader(c, req, info))
			resp, err := upstream.Client().Do(req)
			require.NoError(t, err)
			id, _, taskErr := adaptor.DoResponse(c, resp, info)
			require.Nil(t, taskErr)
			require.Equal(t, "task_upstream", id)
			resp, err = adaptor.FetchTask(upstream.URL, "test-key", map[string]any{"task_id": id}, "")
			require.NoError(t, err)
			result, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()
			task, err := adaptor.ParseTaskResult(result)
			require.NoError(t, err)
			require.Equal(t, model.TaskStatusSuccess, task.Status)
			require.Equal(t, 1, creates)
			require.Equal(t, 1, queries)
			expected := map[string]int{"text": 0, "first_frame": 1, "first_last": 2, "reference": 1}[mode]
			require.Equal(t, expected, uploads)
		})
	}
}

func TestAICopyInvalidParametersFailLocally(t *testing.T) {
	for _, body := range []string{
		`{"model":"【稳定】sd2.5-720p（按秒）","prompt":"ocean","seconds":30}`,
		`{"model":"【稳定】sd2.5-720p（按秒）","prompt":"ocean","seconds":15,"duration":4}`,
		`{"model":"sd2.0-480fast-ad渠道16x9","prompt":"ocean","seconds":15,"extra":{"aspect_ratio":"9:16"}}`,
		`{"model":"官方h3-2k","prompt":"ocean","seconds":5,"mode":"reference"}`,
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		defer common.CleanupBodyStorage(c)
		info := newPlaygroundSoraInfo()
		info.ChannelType = constant.ChannelTypeOpenAI
		e := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
		require.NotNil(t, e)
		require.True(t, e.LocalError)
		require.Equal(t, http.StatusBadRequest, e.StatusCode)
	}
}

func TestAICopyReferenceUploadFailureStopsSubmission(t *testing.T) {
	service.InitHttpClient()
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/v1/uploads", r.URL.Path)
		w.WriteHeader(503)
	}))
	defer upstream.Close()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "ocean", InputReference: "data:image/png;base64,aW1hZ2U="})
	info := newPlaygroundSoraInfo()
	info.ChannelBaseUrl = upstream.URL
	a := &TaskAdaptor{}
	a.Init(info)
	_, err := a.buildAICopyVideoBody(c, info)
	require.ErrorContains(t, err, "upload returned HTTP 503")
	require.Equal(t, 1, calls)
}

func TestAICopyAudioVideoReferencesAndExplicitZero(t *testing.T) {
	service.InitHttpClient()
	kinds := map[string]int{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/uploads", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		defer r.MultipartForm.RemoveAll()
		for kind := range r.MultipartForm.File {
			kinds[kind]++
		}
		_, _ = io.WriteString(w, `{"url":"https://cdn.example/reference"}`)
	}))
	defer upstream.Close()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "ocean", Duration: 6, Metadata: map[string]any{
		"seed": 0, "watermark": false,
		"reference_videos": []map[string]string{{"url": "data:video/mp4;base64,dmlkZW8="}},
		"reference_audios": []map[string]string{{"url": "data:audio/wav;base64,YXVkaW8="}},
	}})
	info := newPlaygroundSoraInfo()
	info.ChannelBaseUrl = upstream.URL
	a := &TaskAdaptor{}
	a.Init(info)
	body, err := a.buildAICopyVideoBody(c, info)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.DecodeJson(body, &payload))
	extra := payload["extra"].(map[string]any)
	require.Equal(t, float64(0), extra["seed"])
	require.Equal(t, false, extra["watermark"])
	require.Equal(t, map[string]int{"video": 1, "audio": 1}, kinds)
	require.Equal(t, "https://cdn.example/reference", extra["reference_audios"].([]any)[0].(map[string]any)["url"])
}

func TestAICopyPublicRequestsInferModelMode(t *testing.T) {
	for _, tc := range []struct{ model, media, mode string }{
		{"happyhorse-1.1-r2v-720p", `"images":["https://cdn.example/a.png"]`, "reference"},
		{"官方h3-720p", `"input_reference":{"url":"https://cdn.example/a.png"}`, "reference"},
		{"omni-fast-视频编辑（带水印）", `"extra":{"reference_videos":[{"url":"https://cdn.example/a.mp4"}]}`, "video_edit"},
		{"【稳定】sd2.0-720满血（按秒）", `"extra":{"reference_images":[{"url":"https://cdn.example/a.png","role":"first_frame"},{"url":"https://cdn.example/b.png","role":"last_frame"}]}`, "first_last"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			body := `{"model":"` + tc.model + `","prompt":"ocean","seconds":10,` + tc.media + `}`
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			defer common.CleanupBodyStorage(c)
			info := newPlaygroundSoraInfo()
			info.ChannelType = constant.ChannelTypeOpenAI
			require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))
			req, err := relaycommon.GetTaskRequest(c)
			require.NoError(t, err)
			require.Equal(t, tc.mode, req.Mode)
		})
	}
}

func TestAICopyMissingDurationUsesSameExplicitValueForBillingAndSubmission(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{"model":"【官方稳定版】sd2.0-480p-满血","prompt":"ocean"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	defer common.CleanupBodyStorage(c)
	info := newPlaygroundSoraInfo()
	info.ChannelType = constant.ChannelTypeOpenAI
	info.OriginModelName = "【官方稳定版】sd2.0-480p-满血"
	info.UpstreamModelName = info.OriginModelName
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	ratios := adaptor.EstimateBilling(c, info)
	body, err := adaptor.buildAICopyVideoBody(c, info)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.DecodeJson(body, &payload))
	require.Greater(t, ratios["seconds"], float64(0))
	require.Equal(t, ratios["seconds"], payload["seconds"])
}
