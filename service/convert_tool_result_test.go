package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestClaudeToOpenAIRequestMovesParallelToolResultImagesToUserMessage(t *testing.T) {
	var request dto.ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"model":"gpt-5.5",
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call-1","name":"read","input":{"path":"a"}},
				{"type":"tool_use","id":"call-2","name":"read","input":{"path":"b"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"call-1","content":[
					{"type":"text","text":"first result"},
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"BASE64_ONE"}}
				]},
				{"type":"tool_result","tool_use_id":"call-2","content":[
					{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"BASE64_TWO"}},
					{"type":"image","source":{"type":"base64","media_type":"image/webp","data":"BASE64_THREE"}}
				]}
			]}
		]
	}`), &request))

	converted, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})
	require.NoError(t, err)
	require.Len(t, converted.Messages, 4)

	require.Equal(t, "assistant", converted.Messages[0].Role)
	require.Len(t, converted.Messages[0].ParseToolCalls(), 2)

	firstTool := converted.Messages[1]
	require.Equal(t, "tool", firstTool.Role)
	require.Equal(t, "call-1", firstTool.ToolCallId)
	require.Equal(t, "first result", firstTool.Content)
	require.NotContains(t, firstTool.Content, "BASE64_ONE")

	secondTool := converted.Messages[2]
	require.Equal(t, "tool", secondTool.Role)
	require.Equal(t, "call-2", secondTool.ToolCallId)
	require.Equal(t, "[Tool result media attached in the following user message.]", secondTool.Content)
	require.NotContains(t, secondTool.Content, "BASE64_TWO")
	require.NotContains(t, secondTool.Content, "BASE64_THREE")

	userMedia := converted.Messages[3]
	require.Equal(t, "user", userMedia.Role)
	contents := userMedia.ParseContent()
	require.Len(t, contents, 5)
	require.Equal(t, "[Tool output media for call call-1]", contents[0].Text)
	require.Equal(t, "data:image/png;base64,BASE64_ONE", contents[1].GetImageMedia().Url)
	require.Equal(t, "[Tool output media for call call-2]", contents[2].Text)
	require.Equal(t, "data:image/jpeg;base64,BASE64_TWO", contents[3].GetImageMedia().Url)
	require.Equal(t, "data:image/webp;base64,BASE64_THREE", contents[4].GetImageMedia().Url)

	payload, err := common.Marshal(converted)
	require.NoError(t, err)
	for _, data := range []string{"BASE64_ONE", "BASE64_TWO", "BASE64_THREE"} {
		require.Equal(t, 1, strings.Count(string(payload), data))
	}
}

func TestClaudeToOpenAIRequestKeepsStringToolResult(t *testing.T) {
	var request dto.ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"model":"gpt-5.5",
		"messages":[{"role":"user","content":[
			{"type":"tool_result","tool_use_id":"call-1","content":"plain output"}
		]}]
	}`), &request))

	converted, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})
	require.NoError(t, err)
	require.Len(t, converted.Messages, 1)
	require.Equal(t, "tool", converted.Messages[0].Role)
	require.Equal(t, "plain output", converted.Messages[0].Content)
}
