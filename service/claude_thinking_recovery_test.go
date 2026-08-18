package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestSanitizeAllClaudeThinkingPreservesVisibleHistoryAndConfig(t *testing.T) {
	body := []byte(`{
		"model":"claude-opus-5",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"custom_top_level":{"keep":true},
		"messages":[
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"hidden","signature":"sig-a"},
				{"type":"text","text":"visible"},
				{"type":"redacted_thinking","data":"opaque-a"},
				{"type":"tool_use","id":"tool-1","name":"read","input":{"path":"a"}}
			]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"ok"}]},
			{"role":"assistant","content":[{"type":"thinking","thinking":"only","signature":"sig-only"}]}
		]
	}`)

	result, err := SanitizeAllClaudeThinking(body)
	require.NoError(t, err)
	require.Equal(t, 3, result.RemovedBlocks)
	require.Equal(t, 1, result.RemovedMessages)
	require.Len(t, result.Fingerprints, 3)

	var decoded map[string]any
	require.NoError(t, common.Unmarshal(result.Body, &decoded))
	require.Equal(t, map[string]any{"type": "enabled", "budget_tokens": float64(2048)}, decoded["thinking"])
	require.Equal(t, map[string]any{"keep": true}, decoded["custom_top_level"])

	messages := decoded["messages"].([]any)
	require.Len(t, messages, 2)
	assistant := messages[0].(map[string]any)
	blocks := assistant["content"].([]any)
	require.Len(t, blocks, 2)
	require.Equal(t, "text", blocks[0].(map[string]any)["type"])
	require.Equal(t, "tool_use", blocks[1].(map[string]any)["type"])
	require.Equal(t, "tool_result", messages[1].(map[string]any)["content"].([]any)[0].(map[string]any)["type"])
}

func TestSanitizeKnownInvalidClaudeThinkingRemovesOnlyRememberedBlocks(t *testing.T) {
	clearInvalidClaudeThinkingTestCache()
	t.Cleanup(clearInvalidClaudeThinkingTestCache)

	block := []byte(`{"type":"thinking","thinking":"a","signature":"sig-a"}`)
	_, fingerprint, ok := claudeThinkingBlockFingerprint(block)
	require.True(t, ok)
	invalidClaudeThinkingCache.Store(fingerprint, invalidClaudeThinkingCacheEntry{
		expiresAt: time.Now().Add(time.Hour).UnixNano(),
	})

	body := []byte(`{"messages":[{"role":"assistant","content":[
		{"type":"thinking","thinking":"a","signature":"sig-a"},
		{"type":"thinking","thinking":"b","signature":"sig-b"},
		{"type":"text","text":"visible"}
	]}]}`)
	result, err := SanitizeKnownInvalidClaudeThinking(body)
	require.NoError(t, err)
	require.Equal(t, 1, result.RemovedBlocks)
	require.NotContains(t, string(result.Body), "sig-a")
	require.Contains(t, string(result.Body), "sig-b")
	require.Contains(t, string(result.Body), "visible")
}

func TestIsInvalidClaudeThinkingSignatureError(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("thinking: Invalid signature in thinking block (request id: upstream)"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)
	require.True(t, IsInvalidClaudeThinkingSignatureError(err))

	err.StatusCode = http.StatusTooManyRequests
	require.False(t, IsInvalidClaudeThinkingSignatureError(err))
}

func clearInvalidClaudeThinkingTestCache() {
	invalidClaudeThinkingCache.Range(func(key, value any) bool {
		invalidClaudeThinkingCache.Delete(key)
		return true
	})
}
