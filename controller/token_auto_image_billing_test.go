package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestPersonalAutoImageReservationAndSettlement(t *testing.T) {
	for _, tc := range []struct {
		name, first, next     string
		wallet, token, actual int
		freeFirst, failure    bool
	}{
		{name: "higher price", first: "a", next: "image", wallet: 1000000, token: 1000000, actual: 50000},
		{name: "lower price refunds excess", first: "image", next: "a", wallet: 1000000, token: 1000000, actual: 5000},
		{name: "free to paid", first: "a", next: "image", wallet: 1000000, token: 1000000, actual: 50000, freeFirst: true},
		{name: "wallet insufficient", first: "a", next: "image", wallet: 10000, token: 1000000, failure: true},
		{name: "token insufficient", first: "a", next: "image", wallet: 1000000, token: 10000, failure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupPersonalAutoTest(t)
			previousPrice := ratio_setting.ModelPrice2JSONString()
			previousTokenGroups := ratio_setting.ImageTokenBillingGroups2JSONString()
			previousSize := ratio_setting.ImageSizeGroupPrices2JSONString()
			previousBatch := common.BatchUpdateEnabled
			common.BatchUpdateEnabled = false
			t.Cleanup(func() {
				_ = ratio_setting.UpdateModelPriceByJSONString(previousPrice)
				_ = ratio_setting.UpdateImageTokenBillingGroupsByJSONString(previousTokenGroups)
				_ = ratio_setting.UpdateImageSizeGroupPricesByJSONString(previousSize)
				common.BatchUpdateEnabled = previousBatch
			})
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":0.1}`))
			require.NoError(t, ratio_setting.UpdateImageTokenBillingGroupsByJSONString(`[]`))
			require.NoError(t, ratio_setting.UpdateImageSizeGroupPricesByJSONString(`{}`))
			if tc.freeFirst {
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"a":0,"image":1}`))
			}
			user := model.User{Id: 71, Username: "auto-image-billing", Quota: tc.wallet, Group: "default"}
			require.NoError(t, db.Create(&user).Error)
			token := seedToken(t, db, user.Id, "image-fixture", "image-billing-fixture")
			require.NoError(t, db.Model(token).Updates(map[string]any{"unlimited_quota": false, "remain_quota": tc.token}).Error)
			c := personalAutoContext(t, []string{tc.first, tc.next}, true, "/v1/images/generations")
			info := &relaycommon.RelayInfo{UserId: user.Id, TokenId: token.Id, TokenKey: token.Key, TokenGroup: "auto", UserGroup: "default", UsingGroup: "auto", OriginModelName: "gpt-image-2", ForcePreConsume: true, Request: &dto.ImageRequest{Model: "gpt-image-2", Size: "1024x1024"}}
			info.UserSetting.BillingPreference = "wallet_only"
			common.SetContextKey(c, constant.ContextKeyAutoGroup, tc.first)
			require.Nil(t, prepareAutoImageAttemptBilling(c, info, 0, &types.TokenCountMeta{}))
			reserved := 0
			original := info.Billing
			if original != nil {
				reserved = original.GetPreConsumedQuota()
			} else {
				require.True(t, tc.freeFirst)
			}
			common.SetContextKey(c, constant.ContextKeyAutoGroup, tc.next)
			apiErr := prepareAutoImageAttemptBilling(c, info, 0, &types.TokenCountMeta{})
			if tc.failure {
				require.NotNil(t, apiErr)
				require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
				require.True(t, types.IsSkipRetryError(apiErr))
				require.Equal(t, reserved, info.Billing.GetPreConsumedQuota())
				require.NoError(t, service.SettleBilling(c, info, 0))
			} else {
				require.Nil(t, apiErr)
				require.NotNil(t, info.Billing)
				if original != nil {
					require.Same(t, original, info.Billing)
				}
				require.Equal(t, max(reserved, tc.actual), info.Billing.GetPreConsumedQuota())
				require.NoError(t, service.SettleBilling(c, info, tc.actual))
				require.NoError(t, service.SettleBilling(c, info, tc.actual)) // idempotent settlement
			}
			require.NoError(t, db.First(&user, user.Id).Error)
			var saved model.Token
			require.NoError(t, db.First(&saved, token.Id).Error)
			require.Equal(t, tc.wallet-tc.actual, user.Quota)
			require.Equal(t, tc.token-tc.actual, saved.RemainQuota)
		})
	}
}
