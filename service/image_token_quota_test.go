package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
	"time"
)

func TestImageTokenPricesSeparateModalitiesAndCachedSubsets(t *testing.T) {
	p := types.ImageTokenPrice{Input: 5, Output: 10, ImageInput: 8, ImageOutput: 30, CachedInput: 1, CachedImageInput: 2, CacheCreation: 6}
	u := &dto.Usage{PromptTokens: 1000, CompletionTokens: 1000, PromptTokensDetails: dto.InputTokenDetails{ImageTokens: 600, CachedTokens: 300, CachedTokensDetails: &dto.CachedTokenDetails{TextTokens: 100, ImageTokens: 200}}, CompletionTokenDetails: dto.OutputTokenDetails{ImageTokens: 900}}
	// 300*5 + 400*8 + 100*1 + 200*2 + 100*10 + 900*30 = 33200.
	require.Equal(t, "0.0332", imageTokenCost(u, p).String())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{StartTime: time.Now(), PriceData: types.PriceData{ImageTokenPrice: &p, ModelRatio: 2.5, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 3.2}, OtherRatios: map[string]float64{"n": 2}}}
	summary := calculateTextQuotaSummary(c, info, u)
	require.Equal(t, int(0.0332*3.2*common.QuotaPerUnit+0.5), summary.Quota)
	// Explicit free input remains free even when output is charged.
	p.Input = 0
	p.CachedInput = 0
	require.Equal(t, "0", imageTokenCost(&dto.Usage{PromptTokens: 100}, p).String())
	require.Equal(t, "0", imageTokenCost(nil, p).String())
}
func TestGeminiFixedSizePricesChargeActualImageCount(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{StartTime: time.Now(), PriceData: types.PriceData{UsePrice: true, ImageSizePriceOverride: true, ModelPrice: 0.15, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}}
	for _, count := range []int{0, 1, 2} {
		usage := &dto.Usage{PromptTokens: 100, CompletionTokens: 200, GeneratedImages: &count}
		got := calculateTextQuotaSummary(c, info, usage)
		require.Equal(t, int(0.15*float64(count)*common.QuotaPerUnit), got.Quota)
	}
}
