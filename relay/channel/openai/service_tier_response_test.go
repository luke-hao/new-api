package openai

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServiceTierObservedAcrossOpenAIResponses(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	const expr = `param("service_tier") == "fast" ? p*4 : p*2`
	cases := []struct {
		name, body string
		stream     bool
	}{
		{"chat", `{"id":"chatcmpl-a","service_tier":"default","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`, false},
		{"responses", `{"id":"resp_a","service_tier":"default","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`, false},
		{"chat_stream", "data: {\"id\":\"chatcmpl-a\",\"service_tier\":\"default\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\ndata: [DONE]\n\n", true},
		{"responses_stream", "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_a\",\"service_tier\":\"default\",\"output\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":2,\"total_tokens\":12}}}\n\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, resp, info := chatIDContext(tc.body, tc.stream)
			info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: expr}
			switch tc.name {
			case "chat":
				_, err := OpenaiHandler(c, info, resp)
				require.Nil(t, err)
			case "responses":
				_, err := OaiResponsesHandler(c, info, resp)
				require.Nil(t, err)
			case "chat_stream":
				_, err := OaiStreamHandler(c, info, resp)
				require.Nil(t, err)
			case "responses_stream":
				_, err := OaiResponsesStreamHandler(c, info, resp)
				require.Nil(t, err)
			}
			require.Equal(t, "default", info.UpstreamServiceTier)
		})
	}
}

func TestResponsesLateUnpricedFastDoesNotSettleEarlierUsage(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	body := "data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_a\",\"usage\":{\"input_tokens\":10,\"output_tokens\":2,\"total_tokens\":12}}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_a\",\"service_tier\":\"priority\",\"output\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":2,\"total_tokens\":12}}}\n\n"
	c, _, resp, info := chatIDContext(body, true)
	info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: "p*2+c*10"}
	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.NotNil(t, err)
	require.Equal(t, types.ErrorCodeModelPriceError, err.GetErrorCode())
	require.Contains(t, err.Error(), "fast 价格未配置")
	require.Nil(t, usage, "unpriced Fast usage must not reach partial base-price settlement")
}
