package service

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
)

func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if preConsumedQuota < 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("pre-consumed quota cannot be negative"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo == nil {
		return fmt.Errorf("relayInfo is nil")
	}
	if actualQuota < 0 {
		return fmt.Errorf("actual quota cannot be negative")
	}
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		if preConsumed < 0 {
			return fmt.Errorf("pre-consumed quota cannot be negative")
		}
		delta, err := common.SafeAddInt("billing quota delta", actualQuota, -preConsumed)
		if err != nil {
			return err
		}

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("post pre-consume charge: %s (actual: %s, pre-consumed: %s)",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("post pre-consume refund: %s (actual: %s, pre-consumed: %s)",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("pre-consume matched actual quota: %s", logger.FormatQuota(actualQuota)))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}

		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, delta, preConsumed)
			}
		}
		return nil
	}

	if relayInfo.FinalPreConsumedQuota < 0 {
		return fmt.Errorf("pre-consumed quota cannot be negative")
	}
	quotaDelta, err := common.SafeAddInt("billing quota delta", actualQuota, -relayInfo.FinalPreConsumedQuota)
	if err != nil {
		return err
	}
	if quotaDelta != 0 {
		return adjustPostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}
