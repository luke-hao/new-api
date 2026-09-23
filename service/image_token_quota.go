package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

// Return USD, excluding group ratios. Cached tokens are subsets of input modalities.
func imageTokenCost(usage *dto.Usage, price types.ImageTokenPrice) decimal.Decimal {
	if usage == nil {
		return decimal.Zero
	}
	input := max(usage.PromptTokens, 0)
	image := min(max(usage.PromptTokensDetails.ImageTokens, 0), input)
	text := input - image
	cached := min(max(usage.PromptTokensDetails.CachedTokens, 0), input)
	cachedImage := 0
	if details := usage.PromptTokensDetails.CachedTokensDetails; details != nil {
		cachedImage = min(max(details.ImageTokens, 0), image, cached)
	} else {
		// Without a modality breakdown allocate cache to text first, then images.
		cachedImage = min(max(cached-text, 0), image)
	}
	cachedText := min(cached-cachedImage, text)
	creation := min(max(usage.PromptTokensDetails.CachedCreationTokens, 0), text-cachedText)
	output := max(usage.CompletionTokens, 0)
	imageOutput := min(max(usage.CompletionTokenDetails.ImageTokens, 0), output)
	cost := decimal.Zero
	for _, lane := range []struct {
		tokens int
		price  float64
	}{
		{text - cachedText - creation, price.Input}, {image - cachedImage, price.ImageInput},
		{cachedText, price.CachedInput}, {cachedImage, price.CachedImageInput}, {creation, price.CacheCreation},
		{output - imageOutput, price.Output}, {imageOutput, price.ImageOutput},
	} {
		cost = cost.Add(decimal.NewFromInt(int64(lane.tokens)).Mul(decimal.NewFromFloat(lane.price)))
	}
	return cost.Div(decimal.NewFromInt(1000000))
}
