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
		"vip": {"image": {"gpt-image-2": {"1K": 0.05, "4K": 0.17}}}
	}`))

	price, ok := GetImageSizeGroupPrice("vip", "image", "gpt-image-2", "4K")
	require.True(t, ok)
	require.InDelta(t, 0.17, price, 1e-12)
	_, ok = GetImageSizeGroupPrice("vip", "image", "gpt-image-2", "2K")
	require.False(t, ok)
	_, ok = GetImageSizeGroupPrice("default", "image", "gpt-image-2", "4K")
	require.False(t, ok)
}

func TestImageSizeGroupPricesRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "negative price", value: `{"vip":{"image":{"gpt-image-2":{"4K":-0.1}}}}`},
		{name: "unknown tier", value: `{"vip":{"image":{"gpt-image-2":{"8K":0.1}}}}`},
		{name: "unknown model", value: `{"vip":{"image":{"other-model":{"4K":0.1}}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseImageSizeGroupPricesJSONString(tt.value)
			require.Error(t, err)
		})
	}
}
