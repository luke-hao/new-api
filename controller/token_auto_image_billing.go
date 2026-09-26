package controller

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Reprice the selected group and fund the same billing session before dispatch.
// Settlement refunds any excess reservation; retries do not create another bill.
func prepareAutoImageAttemptBilling(c *gin.Context, info *relaycommon.RelayInfo, tokens int, meta *types.TokenCountMeta) *types.NewAPIError {
	price, err := helper.ModelPriceHelper(c, info, tokens, meta)
	if err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry())
	}
	if info.Billing != nil {
		if err := info.Billing.Reserve(price.QuotaToPreConsume); err != nil {
			return types.NewError(err, types.ErrorCodeInsufficientUserQuota, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusForbidden))
		}
	} else if !price.FreeModel {
		// A free first group may fail over to a paid group.
		return service.PreConsumeBilling(c, price.QuotaToPreConsume, info)
	}
	return nil
}
