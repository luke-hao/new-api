package sora

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundAICopyMockUpstream(t *testing.T) {
	for _, upload := range []bool{false, true} {
		name := "json"
		if upload {
			name = "multipart"
		}
		t.Run(name, func(t *testing.T) {
			model := "sd2.0-1080满血-ad渠道9x16"
			var body bytes.Buffer
			contentType := "application/json"
			if upload {
				writer := multipart.NewWriter(&body)
				for key, value := range map[string]string{"model": model, "prompt": "ocean sunrise", "group": "视频生成", "mode": "first_frame", "seconds": "15", "duration": "15", "size": "1080p", "extra": `{"aspect_ratio":"9:16","resolution":"1080p"}`} {
					require.NoError(t, writer.WriteField(key, value))
				}
				header := make(textproto.MIMEHeader)
				header.Set("Content-Disposition", `form-data; name="input_reference"; filename="frame.png"`)
				header.Set("Content-Type", "image/png")
				file, err := writer.CreatePart(header)
				require.NoError(t, err)
				_, err = file.Write([]byte("\x89PNG\r\n\x1a\nfixture"))
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				contentType = writer.FormDataContentType()
			} else {
				data, err := common.Marshal(map[string]any{"model": model, "prompt": "ocean sunrise", "group": "视频生成", "mode": "text", "duration": 15, "seconds": 15, "size": "1080p", "extra": map[string]string{"aspect_ratio": "9:16", "resolution": "1080p"}})
				require.NoError(t, err)
				body.Write(data)
			}
			received := false
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v1/videos", r.URL.Path)
				if upload {
					require.NoError(t, r.ParseMultipartForm(1<<20))
					defer r.MultipartForm.RemoveAll()
					require.Equal(t, model, r.FormValue("model"))
					require.Equal(t, "1080p", r.FormValue("size"))
					require.Equal(t, "15", r.FormValue("seconds"))
					require.JSONEq(t, `{"aspect_ratio":"9:16","resolution":"1080p"}`, r.FormValue("extra"))
					require.Empty(t, r.FormValue("group"))
					require.Len(t, r.MultipartForm.File["input_reference"], 1)
				} else {
					var payload map[string]any
					require.NoError(t, common.DecodeJson(r.Body, &payload))
					require.Equal(t, model, payload["model"])
					require.Equal(t, "1080p", payload["size"])
					require.Equal(t, float64(15), payload["seconds"])
					require.Equal(t, map[string]any{"aspect_ratio": "9:16", "resolution": "1080p"}, payload["extra"])
					require.NotContains(t, payload, "group")
					require.NotContains(t, payload, "duration")
				}
				received = true
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"fixture-task","status":"queued"}`)
			}))
			defer upstream.Close()
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", &body)
			ctx.Request.Header.Set("Content-Type", contentType)
			defer common.CleanupBodyStorage(ctx)
			info := newPlaygroundSoraInfo()
			info.ChannelType = constant.ChannelTypeOpenAI
			info.UpstreamModelName = model
			info.ChannelBaseUrl = upstream.URL
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
			payload, err := adaptor.BuildRequestBody(ctx, info)
			require.NoError(t, err)
			url, err := adaptor.BuildRequestURL(info)
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, url, payload)
			require.NoError(t, err)
			require.NoError(t, adaptor.BuildRequestHeader(ctx, req, info))
			response, err := upstream.Client().Do(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.True(t, received)
		})
	}
}
