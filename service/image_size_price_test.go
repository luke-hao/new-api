package service

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageSizeFixedPriceStillMultipliesImageCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		PriceData: types.PriceData{
			UsePrice:               true,
			ModelPrice:             0.17,
			ImageSizePriceOverride: true,
			ImageSizePriceTier:     "4K",
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
			OtherRatios: map[string]float64{"n": 3},
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, &dto.Usage{
		PromptTokens: 1,
		TotalTokens:  1,
	})
	require.Equal(t, int(math.Round(0.17*common.QuotaPerUnit*3)), summary.Quota)
}
