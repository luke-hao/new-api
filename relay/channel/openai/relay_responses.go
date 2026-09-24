package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if err := helper.ObserveServiceTierBilling(info, responseBody); err != nil {
		return nil, err
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
		}
	}
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	var responseTextBuilder strings.Builder
	var streamErr *types.NewAPIError
	type pendingEvent struct {
		event dto.ResponsesStreamResponse
		data  string
	}
	var pending []pendingEvent
	pendingBytes := 0
	committed, terminal := false, false

	emit := func(event dto.ResponsesStreamResponse, data string) {
		if !committed {
			committed = true
			for _, item := range pending {
				sendResponsesStreamData(c, item.event, item.data)
			}
			pending = nil
		}
		sendResponsesStreamData(c, event, data)
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if err := helper.ObserveServiceTierBilling(info, common.StringToByteSlice(data)); err != nil {
			streamErr = err
			sr.Stop(err)
			return
		}
		var event dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			streamErr = types.NewErrorWithStatusCode(fmt.Errorf("invalid Responses stream event: %w", err), types.ErrorCodeIncompleteStream, http.StatusBadGateway)
			sr.Stop(streamErr)
			return
		}
		if response := event.Response; response != nil && response.Usage != nil {
			// Preserve observed usage from failure/incomplete events as well as completed.
			usage.PromptTokens = response.Usage.InputTokens
			usage.CompletionTokens = response.Usage.OutputTokens
			usage.TotalTokens = response.Usage.TotalTokens
			if response.Usage.InputTokensDetails != nil {
				usage.PromptTokensDetails.CachedTokens = response.Usage.InputTokensDetails.CachedTokens
			}
		}
		if failure := responsesStreamFailure(event, data); failure != nil {
			streamErr = failure
			sr.Stop(failure)
			return
		}
		if event.Type == "" {
			streamErr = types.NewErrorWithStatusCode(fmt.Errorf("Responses event has no type"), types.ErrorCodeIncompleteStream, http.StatusBadGateway)
			sr.Stop(streamErr)
			return
		}

		// Keep the failed attempt's IDs and lifecycle events out of the client stream.
		// A bounded preface preserves streaming and never buffers the answer itself.
		if !committed && isResponsesPreface(event) && len(pending) < 64 && pendingBytes+len(data) <= 64<<10 {
			pending = append(pending, pendingEvent{event, data})
			pendingBytes += len(data)
			return
		}

		emit(event, data)
		switch event.Type {
		case "response.completed", "response.incomplete":
			terminal = true
			if event.Response != nil && event.Response.HasImageGenerationCall() {
				c.Set("image_generation_call", true)
				c.Set("image_generation_call_quality", event.Response.GetQuality())
				c.Set("image_generation_call_size", event.Response.GetSize())
			}
			sr.Done()
		case "response.output_text.delta":
			responseTextBuilder.WriteString(event.Delta)
		case dto.ResponsesOutputTypeItemDone:
			if event.Item != nil && event.Item.Type == dto.BuildInCallWebSearchCall &&
				info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
				if tool, ok := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; ok && tool != nil {
					tool.CallCount++
				}
			}
		}
	})

	if streamErr == nil && !terminal {
		reason := "Responses stream ended without a terminal response"
		if info.StreamStatus != nil {
			reason += ": " + info.StreamStatus.Summary()
		}
		streamErr = types.NewErrorWithStatusCode(fmt.Errorf("%s", reason), types.ErrorCodeIncompleteStream, http.StatusBadGateway)
	}
	if c.Request.Context().Err() != nil {
		streamErr = types.NewErrorWithStatusCode(c.Request.Context().Err(), types.ErrorCodeIncompleteStream, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
	}
	if streamErr != nil {
		if info.StreamStatus != nil {
			info.StreamStatus.RecordError(streamErr.Error())
		}
		// Output/tool events and confirmed usage must never be replayed. No estimates
		// are charged for an interrupted attempt; settlement uses upstream usage only.
		if committed || service.HasBillableClaudeUsage(usage) {
			types.ErrOptionWithSkipRetry()(streamErr)
		}
		// A missing Fast price must not fall into partial settlement at base price.
		if streamErr.GetErrorCode() == types.ErrorCodeModelPriceError {
			return nil, streamErr
		}
		return usage, streamErr
	}
	if usage.CompletionTokens == 0 && responseTextBuilder.Len() > 0 {
		usage.CompletionTokens = service.CountTextToken(responseTextBuilder.String(), info.UpstreamModelName)
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage, nil
}
