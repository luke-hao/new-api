package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeUsageFromOpenAIUsageUsesUncachedInputTokens(t *testing.T) {
	tests := []struct {
		name                string
		usage               *dto.Usage
		expectedInput       int
		expectedCacheRead   int
		expectedCacheCreate int
	}{
		{
			name: "cache read from production report",
			usage: &dto.Usage{
				PromptTokens:     103396,
				CompletionTokens: 418,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 102912,
				},
			},
			expectedInput:     484,
			expectedCacheRead: 102912,
		},
		{
			name: "cache read and creation",
			usage: &dto.Usage{
				PromptTokens:     1000,
				CompletionTokens: 25,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         200,
					CachedCreationTokens: 300,
				},
				ClaudeCacheCreation5mTokens: 100,
				ClaudeCacheCreation1hTokens: 200,
			},
			expectedInput:       500,
			expectedCacheRead:   200,
			expectedCacheCreate: 300,
		},
		{
			name: "no cache",
			usage: &dto.Usage{
				PromptTokens:     750,
				CompletionTokens: 10,
			},
			expectedInput: 750,
		},
		{
			name: "malformed cache total is clamped",
			usage: &dto.Usage{
				PromptTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 120,
				},
			},
			expectedInput:     0,
			expectedCacheRead: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalUsage := *tt.usage
			converted := buildClaudeUsageFromOpenAIUsage(tt.usage)

			require.Equal(t, tt.expectedInput, converted.InputTokens)
			require.Equal(t, tt.usage.CompletionTokens, converted.OutputTokens)
			require.Equal(t, tt.expectedCacheRead, converted.CacheReadInputTokens)
			require.Equal(t, tt.expectedCacheCreate, converted.CacheCreationInputTokens)
			require.Equal(t, originalUsage, *tt.usage)
		})
	}
}

func TestBuildClaudeUsageFromOpenAIUsageHandlesNil(t *testing.T) {
	require.Nil(t, buildClaudeUsageFromOpenAIUsage(nil))
}
