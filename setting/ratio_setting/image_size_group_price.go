package ratio_setting

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
)

const GPTImage2ModelName = "gpt-image-2"

type ImageSizeGroupPrices map[string]map[string]map[string]map[string]float64

var (
	imageSizeGroupPricesMu sync.RWMutex
	imageSizeGroupPrices   = make(ImageSizeGroupPrices)
)

func ParseImageSizeGroupPricesJSONString(jsonStr string) (ImageSizeGroupPrices, error) {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "{}"
	}

	prices := make(ImageSizeGroupPrices)
	if err := json.Unmarshal([]byte(jsonStr), &prices); err != nil {
		return nil, err
	}
	if prices == nil {
		prices = make(ImageSizeGroupPrices)
	}
	if err := ValidateImageSizeGroupPrices(prices); err != nil {
		return nil, err
	}
	return prices, nil
}

func ValidateImageSizeGroupPrices(prices ImageSizeGroupPrices) error {
	for userGroup, usingGroups := range prices {
		if strings.TrimSpace(userGroup) == "" {
			return fmt.Errorf("image size price user group cannot be empty")
		}
		for usingGroup, models := range usingGroups {
			if strings.TrimSpace(usingGroup) == "" {
				return fmt.Errorf("image size price billing group cannot be empty")
			}
			for modelName, tiers := range models {
				if modelName != GPTImage2ModelName {
					return fmt.Errorf("image size prices only support model %s: %s", GPTImage2ModelName, modelName)
				}
				for tier, price := range tiers {
					switch tier {
					case "1K", "2K", "4K":
					default:
						return fmt.Errorf("unsupported image size price tier: %s", tier)
					}
					if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
						return fmt.Errorf("image size price must be finite and not negative: %s -> %s -> %s -> %s", userGroup, usingGroup, modelName, tier)
					}
				}
			}
		}
	}
	return nil
}

func ImageSizeGroupPrices2JSONString() string {
	imageSizeGroupPricesMu.RLock()
	defer imageSizeGroupPricesMu.RUnlock()

	data, err := json.Marshal(imageSizeGroupPrices)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func UpdateImageSizeGroupPricesByJSONString(jsonStr string) error {
	prices, err := ParseImageSizeGroupPricesJSONString(jsonStr)
	if err != nil {
		return err
	}

	imageSizeGroupPricesMu.Lock()
	imageSizeGroupPrices = prices
	imageSizeGroupPricesMu.Unlock()
	return nil
}

func GetImageSizeGroupPrice(userGroup, usingGroup, modelName, tier string) (float64, bool) {
	if tier == "" {
		return 0, false
	}

	imageSizeGroupPricesMu.RLock()
	defer imageSizeGroupPricesMu.RUnlock()

	usingGroups, ok := imageSizeGroupPrices[userGroup]
	if !ok {
		return 0, false
	}
	models, ok := usingGroups[usingGroup]
	if !ok {
		return 0, false
	}
	tiers, ok := models[modelName]
	if !ok {
		return 0, false
	}
	price, ok := tiers[tier]
	return price, ok
}
