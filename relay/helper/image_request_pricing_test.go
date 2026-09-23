package helper

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestNanoBananaGroupPricingUsesNativeAndCompatibleImageSize(t *testing.T) {
	before := ratio_setting.ImageSizeGroupPrices2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(before)) })
	require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(`{"default":{"生图分组-nanobanana":{"nano-banana-pro":{"2K":0.15,"4K":0.3}}}}`))
	for _, request := range []dto.Request{
		&dto.GeminiChatRequest{GenerationConfig: dto.GeminiChatGenerationConfig{ImageConfig: []byte(`{"imageSize":"2K"}`)}},
		&dto.GeminiChatRequest{GenerationConfig: dto.GeminiChatGenerationConfig{ImageConfig: []byte(`{"image_size":"2K"}`)}},
		&dto.GeneralOpenAIRequest{Model: "nano-banana-pro", ExtraBody: []byte(`{"google":{"image_config":{"image_size":"2K"}}}`)},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := &relaycommon.RelayInfo{OriginModelName: "nano-banana-pro", UserGroup: "default", UsingGroup: "生图分组-nanobanana", Request: request}
		price, err := ModelPriceHelper(c, info, 50, &types.TokenCountMeta{})
		require.NoError(t, err)
		require.True(t, price.UsePrice)
		require.True(t, price.ImageSizePriceOverride)
		require.Equal(t, "2K", price.ImageSizePriceTier)
		require.Equal(t, 0.15, price.ModelPrice)
		require.Equal(t, 1.0, price.GroupRatioInfo.GroupRatio)
		info.OriginModelName = "gemini-3-pro-image-4k"
		require.Equal(t, "4K", imageRequestPricingMeta(info, &types.TokenCountMeta{}).ImagePriceTier)
	}
}
