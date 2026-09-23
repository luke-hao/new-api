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
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"生图分组-image":2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"生图分组-image":3}}`))
	require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(`{
		"vip":{"生图分组-image":{"gpt-image-2":{"4K":0.17}}}
	}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		UserGroup:       "vip",
		UsingGroup:      "生图分组-image",
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

func TestImageModelsUseTokenBillingOnlyInConfiguredGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalRatios := ratio_setting.ModelRatio2JSONString()
	originalCompletion := ratio_setting.CompletionRatio2JSONString()
	originalCache := ratio_setting.CacheRatio2JSONString()
	originalCreateCache := ratio_setting.CreateCacheRatio2JSONString()
	originalImage := ratio_setting.ImageRatio2JSONString()
	originalSizePrices := ratio_setting.ImageSizeGroupPrices2JSONString()
	originalTokenGroups := ratio_setting.ImageTokenBillingGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(originalCompletion))
		require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(originalCache))
		require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(originalCreateCache))
		require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(originalImage))
		require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(originalSizePrices))
		require.NoError(t, ratio_setting.UpdateImageTokenBillingGroupsByJSONString(originalTokenGroups))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":0.06,"gpt-image-2.5-flare":0.1,"gpt-image-2.5-sunburst":0.1,"dall-e-3":0.04}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-image-2":2.5,"gpt-image-2.5-flare":2.5,"gpt-image-2.5-sunburst":2.5}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"gpt-image-2":6,"gpt-image-2.5-flare":6,"gpt-image-2.5-sunburst":6}`))
	require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(`{"gpt-image-2":0.8,"gpt-image-2.5-flare":0.4,"gpt-image-2.5-sunburst":0.4}`))
	require.NoError(t, ratio_setting.UpdateCreateCacheRatioByJSONString(`{"gpt-image-2":2,"gpt-image-2.5-flare":0.4,"gpt-image-2.5-sunburst":0.4}`))
	require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(`{"gpt-image-2":1.6,"gpt-image-2.5-flare":1.6,"gpt-image-2.5-sunburst":1.6}`))
	require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(`{"default":{"生图分组-image":{"gpt-image-2":{"4K":0.17}}}}`))
	require.NoError(t, ratio_setting.UpdateImageTokenBillingGroupsByJSONString(`["OpenAI官key"]`))

	for _, tc := range []struct {
		model       string
		cacheRatio  float64
		createRatio float64
	}{
		{"gpt-image-2", 0.8, 2},
		{"gpt-image-2.5-flare", 0.4, 0.4},
		{"gpt-image-2.5-sunburst", 0.4, 0.4},
	} {
		t.Run(tc.model, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{OriginModelName: tc.model, UserGroup: "default", UsingGroup: "OpenAI官key"}
			price, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{ImagePriceTier: "4K", ImagePriceRatio: 10.0 / 3.0})
			require.NoError(t, err)
			require.False(t, price.UsePrice)
			require.False(t, price.ImageSizePriceOverride)
			require.Equal(t, 2.5, price.ModelRatio)
			require.Equal(t, 6.0, price.CompletionRatio)
			require.Equal(t, tc.cacheRatio, price.CacheRatio)
			require.Equal(t, tc.createRatio, price.CacheCreationRatio)
			require.Equal(t, 1.6, price.ImageRatio)
		})
	}

	ctxAuto, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctxAuto.Set("auto_group", "OpenAI官key")
	infoAuto := &relaycommon.RelayInfo{OriginModelName: "gpt-image-2", UserGroup: "default", UsingGroup: "auto"}
	autoPrice, err := ModelPriceHelper(ctxAuto, infoAuto, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.False(t, autoPrice.UsePrice)
	require.Equal(t, "OpenAI官key", infoAuto.UsingGroup)

	for _, modelName := range []string{"gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst"} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := &relaycommon.RelayInfo{OriginModelName: modelName, UserGroup: "default", UsingGroup: "default"}
		imagePriceRatio := 1.0
		if modelName == "gpt-image-2" {
			imagePriceRatio = 10.0 / 3.0
		}
		price, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{ImagePriceTier: "4K", ImagePriceRatio: imagePriceRatio})
		require.NoError(t, err)
		require.True(t, price.UsePrice)
		if modelName == "gpt-image-2" {
			require.InDelta(t, 0.2, price.ModelPrice, 1e-12)
		} else {
			require.Equal(t, 0.1, price.ModelPrice)
		}
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{OriginModelName: "dall-e-3", UserGroup: "default", UsingGroup: "OpenAI官key"}
	price, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.True(t, price.UsePrice)
	require.Equal(t, 0.04, price.ModelPrice)
}
