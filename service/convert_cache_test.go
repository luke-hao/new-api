package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestClaudeToOpenAIRequestStabilizesPromptCacheInputs(t *testing.T) {
	request := dto.ClaudeRequest{
		Model:    "gpt-5.5",
		Metadata: json.RawMessage(`{"user_id":"session-123","trace_id":"ignored"}`),
		Tools: []dto.Tool{
			{
				Name:        "zeta_lookup",
				Description: "Zeta lookup",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Name:        "alpha_lookup",
				Description: "Alpha lookup",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}

	converted, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	})
	require.NoError(t, err)
	require.Len(t, converted.PromptCacheKey, 63)
	require.True(t, strings.HasPrefix(converted.PromptCacheKey, "claude_"))
	require.NotContains(t, converted.PromptCacheKey, "session-123")
	require.Equal(t, "alpha_lookup", converted.Tools[0].Function.Name)
	require.Equal(t, "zeta_lookup", converted.Tools[1].Function.Name)

	reversed := request
	reversed.Tools = []dto.Tool{
		request.Tools.([]dto.Tool)[1],
		request.Tools.([]dto.Tool)[0],
	}
	reconverted, err := ClaudeToOpenAIRequest(reversed, &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	})
	require.NoError(t, err)
	require.Equal(t, converted.PromptCacheKey, reconverted.PromptCacheKey)
	require.Equal(t, converted.Tools, reconverted.Tools)
}

func TestClaudeMetadataPromptCacheKeyRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata json.RawMessage
	}{
		{name: "empty"},
		{name: "malformed", metadata: json.RawMessage(`{"user_id":`)},
		{name: "missing user id", metadata: json.RawMessage(`{"trace_id":"trace-1"}`)},
		{name: "blank user id", metadata: json.RawMessage(`{"user_id":"  "}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, claudeMetadataPromptCacheKey(tt.metadata))
		})
	}
}

func TestClaudeToOpenAIRequestLeavesOtherModelsUnchanged(t *testing.T) {
	request := dto.ClaudeRequest{
		Model:    "gpt-5.5",
		Metadata: json.RawMessage(`{"user_id":"session-123"}`),
		Tools: []dto.Tool{
			{Name: "zeta_lookup", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "alpha_lookup", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}

	converted, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	})
	require.NoError(t, err)
	require.Empty(t, converted.PromptCacheKey)
	require.Equal(t, "zeta_lookup", converted.Tools[0].Function.Name)
	require.Equal(t, "alpha_lookup", converted.Tools[1].Function.Name)
}
