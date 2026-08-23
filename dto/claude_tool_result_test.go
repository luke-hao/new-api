package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestClaudeRequestTokenCountMetaTreatsToolResultImagesAsFiles(t *testing.T) {
	var request ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"model":"gpt-5.5",
		"messages":[{"role":"user","content":[
			{"type":"tool_result","tool_use_id":"call-1","content":[
				{"type":"text","text":"first result"},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"BASE64_ONE"}},
				{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"BASE64_TWO"}}
			]},
			{"type":"tool_result","tool_use_id":"call-2","content":"plain output"}
		]}]
	}`), &request))

	meta := request.GetTokenCountMeta()
	require.Equal(t, 1, meta.MessagesCount)
	require.Contains(t, meta.CombineText, "first result")
	require.Contains(t, meta.CombineText, "plain output")
	require.NotContains(t, meta.CombineText, "BASE64_ONE")
	require.NotContains(t, meta.CombineText, "BASE64_TWO")
	require.Len(t, meta.Files, 2)
	for _, file := range meta.Files {
		require.Equal(t, types.FileTypeImage, file.FileType)
		require.NotNil(t, file.Source)
	}
}
