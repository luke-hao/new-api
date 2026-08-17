package helper

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func TestModelPriceHelperUsesExactImageSizeGroupPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalModelPrices := ratio_setting.ModelPrice2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	originalImageSizePrices := ratio_setting.ImageSizeGroupPrices2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatios))
		require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(originalImageSizePrices))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":0.06}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"image":2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"image":3}}`))
	require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(`{
		"vip":{"image":{"gpt-image-2":{"4K":0.17}}}
	}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		UserGroup:       "vip",
		UsingGroup:      "image",
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, &types.TokenCountMeta{
		ImagePriceRatio: 10.0 / 3.0,
		ImagePriceTier:  "4K",
	})
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.True(t, priceData.ImageSizePriceOverride)
	require.Equal(t, "4K", priceData.ImageSizePriceTier)
	require.InDelta(t, 0.17, priceData.ModelPrice, 1e-12)
	require.InDelta(t, 1, priceData.GroupRatioInfo.GroupRatio, 1e-12)
	require.Equal(t, int(math.Round(0.17*common.QuotaPerUnit)), priceData.QuotaToPreConsume)

	priceData, err = ModelPriceHelper(ctx, info, 0, &types.TokenCountMeta{
		ImagePriceRatio: 2.5,
		ImagePriceTier:  "2K",
	})
	require.NoError(t, err)
	require.False(t, priceData.ImageSizePriceOverride)
	require.InDelta(t, 0.15, priceData.ModelPrice, 1e-12)
	require.InDelta(t, 3, priceData.GroupRatioInfo.GroupRatio, 1e-12)
}
