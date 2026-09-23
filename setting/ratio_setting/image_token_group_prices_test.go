package ratio_setting

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestImageTokenGroupPricesValidationAndIsolation(t *testing.T) {
	original := ImageTokenGroupPrices2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateImageTokenGroupPricesByJSONString(original)) })
	valid := `{"group-a":{"model":{"input":0,"output":2,"image_input":3,"image_output":4,"cached_input":0,"cached_image_input":1,"cache_creation":0}}}`
	require.NoError(t, UpdateImageTokenGroupPricesByJSONString(valid))
	price, ok := GetImageTokenGroupPrice("group-a", "model")
	require.True(t, ok)
	require.Equal(t, 0.0, price.Input)
	require.Equal(t, 4.0, price.ImageOutput)
	_, ok = GetImageTokenGroupPrice("group-b", "model")
	require.False(t, ok)
	for _, bad := range []string{`{"g":{"m":{}}}`, `{"g":{"m":{"input":null}}}`, `{"g":{"m":{"input":-1}}}`, `[]`} {
		require.Error(t, UpdateImageTokenGroupPricesByJSONString(bad))
		preserved, ok := GetImageTokenGroupPrice("group-a", "model")
		require.True(t, ok)
		require.Equal(t, price, preserved)
	}
}
