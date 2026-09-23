package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageTokenBillingGroups(t *testing.T) {
	original := ImageTokenBillingGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateImageTokenBillingGroupsByJSONString(original))
	})
	require.NoError(t, UpdateImageTokenBillingGroupsByJSONString(`["OpenAI官key"]`))
	require.True(t, IsImageTokenBillingGroup("OpenAI官key"))
	require.False(t, IsImageTokenBillingGroup("default"))
	require.JSONEq(t, `["OpenAI官key"]`, ImageTokenBillingGroups2JSONString())
	for _, input := range []string{`["OpenAI官key","OpenAI官key"]`, `[""]`, `[" bad "]`, `{}`} {
		_, err := ParseImageTokenBillingGroups(input)
		require.Error(t, err)
	}
}
