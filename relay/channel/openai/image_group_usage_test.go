package openai

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestImageGroupUsageKeepsCachedModalitiesAndOutputSplit(t *testing.T) {
	var usage dto.Usage
	require.NoError(t, common.UnmarshalJsonStr(`{"input_tokens":1000,"output_tokens":500,"input_tokens_details":{"text_tokens":400,"image_tokens":600,"cached_tokens":300,"cached_tokens_details":{"text_tokens":100,"image_tokens":200}},"output_tokens_details":{"text_tokens":50,"image_tokens":450}}`, &usage))
	normalizeOpenAIUsage(&usage)
	require.Equal(t, 1000, usage.PromptTokens)
	require.Equal(t, 500, usage.CompletionTokens)
	require.Equal(t, 200, usage.PromptTokensDetails.CachedTokensDetails.ImageTokens)
	require.Equal(t, 450, usage.CompletionTokenDetails.ImageTokens)
	fallback := dto.Usage{OutputTokens: 500}
	normalizeOpenAIUsage(&fallback)
	require.Equal(t, 500, fallback.CompletionTokenDetails.ImageTokens)
}
