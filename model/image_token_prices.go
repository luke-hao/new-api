package model

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

// Copy legacy rates once; global model edits must never change group prices.
func bootstrapImageTokenGroupPrices() error {
	var existing Option
	err := DB.Where(&Option{Key: "ImageTokenGroupPrices"}).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var groups []string
	if err := common.UnmarshalJsonStr(ratio_setting.ImageTokenBillingGroups2JSONString(), &groups); err != nil {
		return err
	}
	prices := ratio_setting.ImageTokenGroupPrices{}
	for _, group := range groups {
		prices[group] = map[string]types.ImageTokenPrice{}
		for _, model := range []string{"gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst"} {
			prices[group][model] = ratio_setting.LegacyImageTokenPrice(model)
		}
	}
	data, err := common.Marshal(prices)
	if err != nil {
		return err
	}
	return UpdateOptionsBulk(map[string]string{"ImageTokenGroupPrices": string(data)})
}
