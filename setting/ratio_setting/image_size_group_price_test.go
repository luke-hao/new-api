package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageSizeGroupPricesRoundTripAndLookup(t *testing.T) {
	original := ImageSizeGroupPrices2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateImageSizeGroupPricesByJSONString(original))
	})

	require.NoError(t, UpdateImageSizeGroupPricesByJSONString(`{
		"vip": {"生图分组-image": {"gpt-image-2": {"1K": 0.05, "4K": 0.17}}}
	}`))

	price, ok := GetImageSizeGroupPrice("vip", "生图分组-image", "gpt-image-2", "4K")
	require.True(t, ok)
	require.InDelta(t, 0.17, price, 1e-12)
	_, ok = GetImageSizeGroupPrice("vip", "生图分组-image", "gpt-image-2", "2K")
	require.False(t, ok)
	_, ok = GetImageSizeGroupPrice("default", "生图分组-image", "gpt-image-2", "4K")
	require.False(t, ok)
	require.NoError(t, UpdateImageSizeGroupPricesByJSONString(`{
		"vip": {"生图分组-image": {"gpt-image-2": {"1K": 0.05}, "gpt-image-2.5-flare": {"1K": 0.09}}}
	}`))
	price, ok = GetImageSizeGroupPrice("vip", "生图分组-image", "gpt-image-2.5-flare", "1K")
	require.True(t, ok)
	require.Equal(t, 0.09, price)
}

func TestImageSizeGroupPricesRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "negative price", value: `{"vip":{"生图分组-image":{"gpt-image-2":{"4K":-0.1}}}}`},
		{name: "unknown tier", value: `{"vip":{"生图分组-image":{"gpt-image-2":{"8K":0.1}}}}`},
		{name: "empty model", value: `{"vip":{"生图分组-image":{"":{"4K":0.1}}}}`},
		{name: "non-image group", value: `{"vip":{"OpenAI官key":{"gpt-image-2":{"4K":0.1}}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseImageSizeGroupPricesJSONString(tt.value)
			require.Error(t, err)
		})
	}
}
