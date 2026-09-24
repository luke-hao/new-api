package billingexpr

import (
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"testing"
)

func TestServiceTierPriceRequiresAnExplicitCondition(t *testing.T) {
	for _, tc := range []struct {
		expression string
		want       bool
	}{
		{`param("service_tier") == "fast" ? p*4 : p*2`, true},
		{`"fast" == param("service_tier") ? p*4 : p*2`, true},
		{`param("service_tier") in ["fast", "priority"] ? p*4 : p*2`, true},
		{`(p*2+c*10) * (param("service_tier") == "fast" ? 1 : 1)`, true},
		{`tier("fast service_tier", p*2)`, false},
		{`param("unrelated") == "fast" ? p*4 : p*2`, false},
		{`param("service_tier") != nil ? p*4 : p*2`, false},
		{`p*2 /* service_tier fast */`, false},
		{`param("service_tier") == "ultrafast" ? p*4 : p*2`, false},
	} {
		t.Run(tc.expression, func(t *testing.T) { require.Equal(t, tc.want, HasServiceTierPrice(tc.expression, "fast")) })
	}
}

func TestServiceTierAliasesPreserveOriginalJSON(t *testing.T) {
	expression := `param("service_tier") == "fast" ? p*4 : p*2`
	input := RequestInput{Body: []byte(`{"service_tier":"priority","id":9007199254740993}`)}
	adjusted, ok := ServiceTierPriceInput(expression, input, "priority")
	require.True(t, ok)
	require.Equal(t, "fast", gjson.GetBytes(adjusted.Body, "service_tier").String())
	require.Equal(t, "priority", gjson.GetBytes(input.Body, "service_tier").String())
	require.Equal(t, "9007199254740993", gjson.GetBytes(adjusted.Body, "id").Raw)
	adjusted, ok = ServiceTierPriceInput(expression, adjusted, "default")
	require.True(t, ok)
	cost, _, err := RunExprWithRequest(expression, TokenParams{P: 100}, adjusted)
	require.NoError(t, err)
	require.Equal(t, 200.0, cost)
}
