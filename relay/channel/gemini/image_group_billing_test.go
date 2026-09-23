package gemini

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiGroupBillingCountsOnlyGeneratedImagesAndRetainsUsage(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	payload := `{"candidates":[{"content":{"role":"model","parts":[{"inlineData":{"mimeType":"image/png","data":"eA=="}},{"inlineData":{"mimeType":"image/png","data":"eQ=="}},{"thought":true,"inlineData":{"mimeType":"image/png","data":"eg=="}},{"inlineData":{"mimeType":"audio/wav","data":"eA=="}}]}}],"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":500,"totalTokenCount":1500,"cachedContentTokenCount":200,"promptTokensDetails":[{"modality":"TEXT","tokenCount":400},{"modality":"IMAGE","tokenCount":600}],"cacheTokensDetails":[{"modality":"IMAGE","tokenCount":200}],"candidatesTokensDetails":[{"modality":"IMAGE","tokenCount":450},{"modality":"TEXT","tokenCount":50}]}}`
	for _, stream := range []bool{false, true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1beta/models/gemini-3-pro-image:generateContent", nil)
		info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatGemini, OriginModelName: "gemini-3-pro-image", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-3-pro-image"}, PriceData: types.PriceData{ImageTokenPrice: &types.ImageTokenPrice{}}}
		body := payload
		if stream {
			body = "data: " + payload + "\n\ndata: [DONE]\n\n"
		}
		response := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
		var usage *dto.Usage
		var apiErr *types.NewAPIError
		if stream {
			usage, apiErr = geminiStreamHandler(c, info, response, func(string, *dto.GeminiChatResponse) bool { return true })
		} else {
			usage, apiErr = GeminiChatHandler(c, info, response)
		}
		require.Nil(t, apiErr)
		require.NotNil(t, usage.GeneratedImages)
		require.Equal(t, 2, *usage.GeneratedImages)
		require.Equal(t, 600, usage.PromptTokensDetails.ImageTokens)
		require.Equal(t, 450, usage.CompletionTokenDetails.ImageTokens)
		require.Equal(t, 200, usage.PromptTokensDetails.CachedTokensDetails.ImageTokens)
	}
}
