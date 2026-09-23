package ratio_setting

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"math"
	"strings"
	"sync"
)

type ImageTokenGroupPrices map[string]map[string]types.ImageTokenPrice

var imageTokenPricesMu sync.RWMutex
var imageTokenPrices = ImageTokenGroupPrices{}

func ParseImageTokenGroupPrices(value string) (ImageTokenGroupPrices, error) {
	if strings.TrimSpace(value) == "" {
		value = "{}"
	}
	var raw map[string]map[string]map[string]*float64
	if err := common.UnmarshalJsonStr(value, &raw); err != nil {
		return nil, err
	}
	result := ImageTokenGroupPrices{}
	for group, models := range raw {
		if strings.TrimSpace(group) == "" || strings.TrimSpace(group) != group {
			return nil, fmt.Errorf("invalid image token price group: %q", group)
		}
		result[group] = map[string]types.ImageTokenPrice{}
		for model, prices := range models {
			if strings.TrimSpace(model) == "" || strings.TrimSpace(model) != model {
				return nil, fmt.Errorf("invalid image token price model: %q", model)
			}
			for _, field := range []string{"input", "output", "image_input", "image_output", "cached_input", "cached_image_input", "cache_creation"} {
				v, ok := prices[field]
				if !ok || v == nil || *v < 0 || math.IsNaN(*v) || math.IsInf(*v, 0) {
					return nil, fmt.Errorf("invalid %s price for %s / %s", field, group, model)
				}
			}
			if len(prices) != 7 {
				return nil, fmt.Errorf("unknown image token price field for %s / %s", group, model)
			}
			data, _ := common.Marshal(prices)
			var price types.ImageTokenPrice
			if err := common.Unmarshal(data, &price); err != nil {
				return nil, err
			}
			result[group][model] = price
		}
	}
	return result, nil
}
func UpdateImageTokenGroupPricesByJSONString(value string) error {
	prices, err := ParseImageTokenGroupPrices(value)
	if err != nil {
		return err
	}
	imageTokenPricesMu.Lock()
	imageTokenPrices = prices
	imageTokenPricesMu.Unlock()
	return nil
}
func ImageTokenGroupPrices2JSONString() string {
	imageTokenPricesMu.RLock()
	defer imageTokenPricesMu.RUnlock()
	data, _ := common.Marshal(imageTokenPrices)
	return string(data)
}
func GetImageTokenGroupPrice(group, model string) (types.ImageTokenPrice, bool) {
	imageTokenPricesMu.RLock()
	defer imageTokenPricesMu.RUnlock()
	price, ok := imageTokenPrices[group][model]
	return price, ok
}

// LegacyImageTokenPrice is only used to migrate the previous group switch.
func LegacyImageTokenPrice(model string) types.ImageTokenPrice {
	ratio, _, _ := GetModelRatio(model)
	input := ratio * 2
	cache, _ := GetCacheRatio(model)
	image, _ := GetImageRatio(model)
	creation, _ := GetCreateCacheRatio(model)
	output := input * GetCompletionRatio(model)
	return types.ImageTokenPrice{Input: input, Output: output, ImageInput: input * image, ImageOutput: output, CachedInput: input * cache, CachedImageInput: input * cache, CacheCreation: input * creation}
}
