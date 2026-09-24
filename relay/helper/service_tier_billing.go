package helper

import (
	"fmt"
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func fastPriceMissing(model string) *types.NewAPIError {
	return types.WithOpenAIError(types.OpenAIError{
		Message: fmt.Sprintf("模型 %s 的 fast 价格未配置，请在模型价格表达式中配置 service_tier 等于 fast 或 priority 的价格规则；Fast price not configured", model),
		Type:    "new_api_error", Code: types.ErrorCodeModelPriceError, Param: "service_tier",
	}, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}

// PrepareServiceTierBilling sees the final upstream JSON and headers, never a
// simulation of filtering/overrides. The frozen expression is preserved while
// its request probe, group and estimate are refreshed for each channel attempt.
func PrepareServiceTierBilling(c *gin.Context, info *relaycommon.RelayInfo, data []byte, headers http.Header) error {
	info.OutboundBillingRequestInput = nil
	info.UpstreamServiceTier = ""
	info.UpstreamClaudeSpeed = ""
	info.RequestedClaudeSpeed = gjson.GetBytes(data, "speed").String()
	info.RequestedServiceTier = gjson.GetBytes(data, "service_tier").String()
	snap := info.TieredBillingSnapshot
	if snap == nil {
		if billingexpr.IsFastServiceTier(info.RequestedServiceTier) {
			return fastPriceMissing(info.OriginModelName)
		}
		return nil
	}
	input := billingexpr.RequestInput{Body: append([]byte(nil), data...), Headers: map[string]string{}}
	for key := range headers {
		// Billing probes never retain upstream authentication material.
		switch http.CanonicalHeaderKey(key) {
		case "Authorization", "Proxy-Authorization", "Cookie", "X-Api-Key", "X-Goog-Api-Key":
			continue
		}
		input.Headers[key] = headers.Get(key)
	}
	if billingexpr.IsFastServiceTier(info.RequestedServiceTier) {
		var ok bool
		input, ok = billingexpr.ServiceTierPriceInput(snap.ExprString, input, info.RequestedServiceTier)
		if !ok {
			return fastPriceMissing(info.OriginModelName)
		}
	}
	cost, trace, err := billingexpr.RunExprWithRequest(snap.ExprString, billingexpr.TokenParams{
		P: float64(snap.EstimatedPromptTokens), C: float64(snap.EstimatedCompletionTokens), Len: float64(snap.EstimatedPromptTokens),
	}, input)
	if err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry())
	}
	ratio := HandleGroupRatio(c, info)
	beforeGroup := cost / 1_000_000 * snap.QuotaPerUnit
	quota, err := common.SafeNonNegativeFloatToInt("outbound tiered quota", math.Round(beforeGroup*ratio.GroupRatio))
	if err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry())
	}
	updated := *snap
	updated.GroupRatio = ratio.GroupRatio
	updated.EstimatedQuotaBeforeGroup = beforeGroup
	updated.EstimatedQuotaAfterGroup = quota
	updated.EstimatedTier = trace.MatchedTier
	info.TieredBillingSnapshot = &updated
	info.OutboundBillingRequestInput = &input
	info.PriceData.GroupRatioInfo = ratio
	info.PriceData.FreeModel = !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume && ratio.GroupRatio == 0
	info.PriceData.QuotaToPreConsume = quota
	return nil
}

// ObserveServiceTierBilling captures response-reported downgrades as well as
// project-default Fast mode. An unexpected unpriced Fast response is an error,
// not permission to charge the standard price. This runs before forwarding.
func ObserveServiceTierBilling(info *relaycommon.RelayInfo, data []byte) *types.NewAPIError {
	tier := gjson.GetBytes(data, "service_tier").String()
	if tier == "" {
		tier = gjson.GetBytes(data, "response.service_tier").String()
	}
	if info == nil || tier == "" {
		return nil
	}
	info.UpstreamServiceTier = tier
	if billingexpr.IsFastServiceTier(tier) {
		snap := info.TieredBillingSnapshot
		if snap == nil || (!billingexpr.HasServiceTierPrice(snap.ExprString, "fast") && !billingexpr.HasServiceTierPrice(snap.ExprString, "priority")) {
			return fastPriceMissing(info.OriginModelName)
		}
	}
	return nil
}

// ObserveClaudeBillingSpeed uses confirmed upstream speed, including the Opus
// 4.6 standard-speed fallback. Missing fields preserve the last observed value.
func ObserveClaudeBillingSpeed(info *relaycommon.RelayInfo, data []byte) {
	if info == nil {
		return
	}
	speed := gjson.GetBytes(data, "usage.speed").String()
	if speed == "" {
		speed = gjson.GetBytes(data, "message.usage.speed").String()
	}
	if speed == "fast" || speed == "standard" {
		info.UpstreamClaudeSpeed = speed
	}
}
