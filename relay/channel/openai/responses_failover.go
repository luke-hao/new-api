package openai

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

// Only lifecycle metadata can be discarded and replayed. Unknown events and all
// output/tool events commit the stream, including provider-specific extensions.
func isResponsesPreface(event dto.ResponsesStreamResponse) bool {
	switch event.Type {
	case "response.created", "response.in_progress", "response.queued":
		return event.Response == nil || (len(event.Response.Output) == 0 && event.Response.Usage == nil)
	case "response.output_item.added":
		return event.Item != nil && event.Item.Type == "message" && len(event.Item.Content) == 0
	case "response.content_part.added":
		return event.Part != nil && event.Part.Type == "output_text" && event.Part.Text == ""
	default:
		return false
	}
}

func responsesStreamFailure(event dto.ResponsesStreamResponse, data string) *types.NewAPIError {
	var envelope struct {
		Error   any    `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = common.UnmarshalJsonStr(data, &envelope)
	upstreamError := dto.GetOpenAIError(envelope.Error)
	if event.Response != nil && event.Response.GetOpenAIError() != nil {
		upstreamError = event.Response.GetOpenAIError()
	}
	failed := event.Type == "error" || event.Type == "response.failed" || event.Type == "response.error"
	if upstreamError == nil && !failed {
		return nil
	}
	if upstreamError != nil {
		switch fmt.Sprint(upstreamError.Code) {
		case "content_filter", "content_policy_violation", "prompt_blocked", "moderation_blocked":
			return types.WithOpenAIError(*upstreamError, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
	}
	message := "upstream Responses stream failed"
	if upstreamError != nil && upstreamError.Message != "" {
		message = upstreamError.Message
	} else if envelope.Message != "" {
		message = envelope.Message
	}
	return types.NewErrorWithStatusCode(fmt.Errorf("%s", message), types.ErrorCodeIncompleteStream, http.StatusBadGateway)
}
