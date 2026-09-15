package openai

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/sjson"
)

// normalizeChatResponseID gives Responses-backed Chat Completions a Chat ID.
// Keep the entire upstream ID after the prefix so it remains traceable without
// retaining a mapping or changing tool call IDs. Patch only the top-level field:
// usage, provider extensions and numeric precision must survive passthrough.
func normalizeChatResponseID(body []byte, info *relaycommon.RelayInfo) []byte {
	if info == nil || info.RelayFormat != types.RelayFormatOpenAI || info.RelayMode != relayconstant.RelayModeChatCompletions {
		return body
	}
	var envelope struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return body
	}
	if envelope.Object != "chat.completion" && envelope.Object != "chat.completion.chunk" {
		return body
	}
	if !strings.HasPrefix(envelope.ID, "resp_") || len(envelope.ID) == len("resp_") {
		return body
	}
	patched, err := sjson.SetBytes(body, "id", "chatcmpl-"+envelope.ID)
	if err != nil {
		return body
	}
	return patched
}
