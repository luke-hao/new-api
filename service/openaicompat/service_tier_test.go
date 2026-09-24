package openaicompat

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChatToResponsesPreservesServiceTier(t *testing.T) {
	for _, tier := range []string{`"fast"`, `"priority"`, `"default"`, ``} {
		request := &dto.GeneralOpenAIRequest{Model: "fixture", ServiceTier: []byte(tier)}
		out, err := ChatCompletionsRequestToResponsesRequest(request)
		require.NoError(t, err)
		if tier == `` {
			require.Empty(t, out.ServiceTier)
		} else {
			require.Equal(t, tier[1:len(tier)-1], out.ServiceTier)
		}
	}
}
