package gemini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertNanoBananaEnablesImageResponseModality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request := dto.GeneralOpenAIRequest{
		Model: "nano-banana-2",
		Messages: []dto.Message{
			{Role: "user", Content: "draw a banana"},
		},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "nano-banana-2",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "nano-banana-2",
		},
	}

	converted, err := CovertOpenAI2Gemini(ctx, request, info)
	require.NoError(t, err)
	require.Equal(t, []string{"TEXT", "IMAGE"}, converted.GenerationConfig.ResponseModalities)
}
