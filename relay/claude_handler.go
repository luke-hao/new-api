package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {

	info.InitChannelMeta(c)

	claudeReq, ok := info.Request.(*dto.ClaudeRequest)

	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.ClaudeRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(claudeReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ClaudeRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	rawClaudeBody, err := bodyStorage.Bytes()
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	var rawBodyOverride []byte
	knownSanitized, err := service.SanitizeKnownInvalidClaudeThinking(rawClaudeBody)
	if err != nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("failed to inspect Claude thinking history: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if knownSanitized.RemovedBlocks > 0 {
		var sanitizedRequest dto.ClaudeRequest
		if err := common.Unmarshal(knownSanitized.Body, &sanitizedRequest); err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("failed to rebuild Claude request after thinking cleanup: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		request = &sanitizedRequest
		rawClaudeBody = knownSanitized.Body
		rawBodyOverride = knownSanitized.Body
		service.MarkClaudeThinkingPreflightRemoved(c, knownSanitized.RemovedBlocks)
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	passThroughEnabled := model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled
	useRawClaudeBody := shouldUseRawClaudeBody(passThroughEnabled, info.ApiType)

	if request.MaxTokens == nil || *request.MaxTokens == 0 {
		defaultMaxTokens := uint(model_setting.GetClaudeSettings().GetDefaultMaxTokens(request.Model))
		request.MaxTokens = &defaultMaxTokens
	}

	if baseModel, effortLevel, ok := reasoning.TrimEffortSuffix(request.Model); ok && effortLevel != "" &&
		(strings.HasPrefix(request.Model, "claude-opus-4-6") ||
			strings.HasPrefix(request.Model, "claude-opus-4-7") ||
			strings.HasPrefix(request.Model, "claude-opus-4-8")) {
		request.Model = baseModel
		request.Thinking = &dto.Thinking{
			Type: "adaptive",
		}
		request.OutputConfig = json.RawMessage(fmt.Sprintf(`{"effort":"%s"}`, effortLevel))
		if strings.HasPrefix(request.Model, "claude-opus-4-7") ||
			strings.HasPrefix(request.Model, "claude-opus-4-8") {
			// Opus 4.7/4.8 reject non-default temperature/top_p/top_k with 400
			// and defaults display to "omitted"; restore the 4.6 visible summary.
			request.Thinking.Display = "summarized"
			request.Temperature = nil
			request.TopP = nil
			request.TopK = nil
		} else {
			request.Temperature = common.GetPointer[float64](1.0)
		}
		info.UpstreamModelName = request.Model
	} else if model_setting.GetClaudeSettings().ThinkingAdapterEnabled &&
		strings.HasSuffix(request.Model, "-thinking") {
		if request.Thinking == nil {
			baseModel := strings.TrimSuffix(request.Model, "-thinking")
			if strings.HasPrefix(baseModel, "claude-opus-4-7") ||
				strings.HasPrefix(baseModel, "claude-opus-4-8") {
				// Opus 4.7/4.8 reject thinking.type="enabled"; use adaptive at high effort.
				request.Thinking = &dto.Thinking{Type: "adaptive", Display: "summarized"}
				request.OutputConfig = json.RawMessage(`{"effort":"high"}`)
				request.Temperature = nil
				request.TopP = nil
				request.TopK = nil
			} else {
				// 因为BudgetTokens 必须大于1024
				if request.MaxTokens == nil || *request.MaxTokens < 1280 {
					request.MaxTokens = common.GetPointer[uint](1280)
				}

				// BudgetTokens 为 max_tokens 的 80%
				budgetTokens, err := common.SafeNonNegativeFloatToInt("thinking budget tokens", float64(*request.MaxTokens)*model_setting.GetClaudeSettings().ThinkingAdapterBudgetTokensPercentage)
				if err != nil {
					return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
				}
				request.Thinking = &dto.Thinking{
					Type:         "enabled",
					BudgetTokens: common.GetPointer[int](budgetTokens),
				}
				// TODO: 临时处理
				// https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#important-considerations-when-using-extended-thinking
				request.Temperature = common.GetPointer[float64](1.0)
			}
		}
		if !model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
			request.Model = strings.TrimSuffix(request.Model, "-thinking")
		}
		info.UpstreamModelName = request.Model
	}

	ensureClaudeAdaptiveThinkingDisplay(request)

	if info.ChannelSetting.SystemPrompt != "" {
		if request.System == nil {
			request.SetStringSystem(info.ChannelSetting.SystemPrompt)
		} else if info.ChannelSetting.SystemPromptOverride {
			common.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
			if request.IsStringSystem() {
				existing := strings.TrimSpace(request.GetStringSystem())
				if existing == "" {
					request.SetStringSystem(info.ChannelSetting.SystemPrompt)
				} else {
					request.SetStringSystem(info.ChannelSetting.SystemPrompt + "\n" + existing)
				}
			} else {
				systemContents := request.ParseSystem()
				newSystem := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
				newSystem.SetText(info.ChannelSetting.SystemPrompt)
				if len(systemContents) == 0 {
					request.System = []dto.ClaudeMediaMessage{newSystem}
				} else {
					request.System = append([]dto.ClaudeMediaMessage{newSystem}, systemContents...)
				}
			}
		}
	}

	if !useRawClaudeBody &&
		service.ShouldChatCompletionsUseResponsesGlobal(info.ChannelId, info.ChannelType, info.OriginModelName) {
		openAIRequest, convErr := service.ClaudeToOpenAIRequest(*request, info)
		if convErr != nil {
			return types.NewError(convErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		usage, newApiErr := chatCompletionsViaResponses(c, info, adaptor, openAIRequest)
		if newApiErr != nil {
			return newApiErr
		}

		service.PostTextConsumeQuota(c, info, usage, nil)
		return nil
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	conversionCount := len(info.RequestConversionChain)
	paramAuditCount := len(info.ParamOverrideAudit)
	usage, newAPIError := executeClaudeAttempt(c, info, adaptor, request, useRawClaudeBody, rawBodyOverride)
	if service.IsInvalidClaudeThinkingSignatureError(newAPIError) {
		recoveredRaw, sanitizeErr := service.SanitizeAllClaudeThinking(rawClaudeBody)
		if sanitizeErr != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("failed to recover Claude thinking history: %w", sanitizeErr), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if recoveredRaw.RemovedBlocks > 0 {
			adjustedBody, marshalErr := common.Marshal(request)
			if marshalErr != nil {
				return types.NewError(marshalErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			recoveredAdjusted, adjustedErr := service.SanitizeAllClaudeThinking(adjustedBody)
			if adjustedErr != nil {
				return types.NewErrorWithStatusCode(fmt.Errorf("failed to rebuild adjusted Claude request: %w", adjustedErr), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			var recoveredRequest dto.ClaudeRequest
			if unmarshalErr := common.Unmarshal(recoveredAdjusted.Body, &recoveredRequest); unmarshalErr != nil {
				return types.NewErrorWithStatusCode(fmt.Errorf("failed to decode recovered Claude request: %w", unmarshalErr), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}

			service.RememberInvalidClaudeThinking(recoveredRaw.Fingerprints)
			service.MarkClaudeThinkingRecoveryAttempt(c, recoveredRaw.RemovedBlocks)
			c.Set(common.UpstreamRequestIdKey, "")
			info.RequestConversionChain = info.RequestConversionChain[:conversionCount]
			info.ParamOverrideAudit = info.ParamOverrideAudit[:paramAuditCount]
			adaptor.Init(info)
			logger.LogInfo(c, fmt.Sprintf("retrying channel #%d after removing %d incompatible Claude thinking blocks", info.ChannelId, recoveredRaw.RemovedBlocks))
			usage, newAPIError = executeClaudeAttempt(c, info, adaptor, &recoveredRequest, useRawClaudeBody, recoveredRaw.Body)
			if newAPIError == nil {
				service.MarkClaudeThinkingRecoverySuccess(c)
			}
		}
	}
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	service.PostTextConsumeQuota(c, info, usage, nil)
	return nil
}

func executeClaudeAttempt(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	adaptor channel.Adaptor,
	request *dto.ClaudeRequest,
	useRawClaudeBody bool,
	rawBodyOverride []byte,
) (*dto.Usage, *types.NewAPIError) {
	var requestBody io.Reader
	if useRawClaudeBody {
		if rawBodyOverride != nil {
			info.UpstreamRequestBodySize = int64(len(rawBodyOverride))
			requestBody = bytes.NewReader(rawBodyOverride)
		} else {
			storage, err := common.GetBodyStorage(c)
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			info.UpstreamRequestBodySize = storage.Size()
			requestBody = common.ReaderOnly(storage)
		}
	} else {
		convertedRequest, err := adaptor.ConvertClaudeRequest(c, info, request)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return nil, newAPIErrorFromParamOverride(err)
			}
		}

		logger.LogDebug(c, "requestBody: %s", jsonData)
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			return nil, newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		return nil, newAPIError
	}
	typedUsage, ok := usage.(*dto.Usage)
	if !ok {
		return nil, types.NewError(fmt.Errorf("invalid Claude usage type %T", usage), types.ErrorCodeBadResponseBody)
	}
	return typedUsage, nil
}

// Opus 4.7/4.8 default adaptive thinking to an omitted display. Preserve an
// explicit caller choice, but make an unspecified display visible as a summary.
func ensureClaudeAdaptiveThinkingDisplay(request *dto.ClaudeRequest) {
	if request == nil || request.Thinking == nil || request.Thinking.Type != "adaptive" || request.Thinking.Display != "" {
		return
	}
	if strings.HasPrefix(request.Model, "claude-opus-4-7") ||
		strings.HasPrefix(request.Model, "claude-opus-4-8") {
		request.Thinking.Display = "summarized"
	}
}

// shouldUseRawClaudeBody only allows raw pass-through when the upstream API
// natively accepts Anthropic Messages payloads. OpenAI-compatible upstreams
// must receive the result of ClaudeToOpenAIRequest; otherwise Anthropic tool
// definitions and tool_choice values are forwarded to an incompatible schema.
func shouldUseRawClaudeBody(passThroughEnabled bool, upstreamAPIType int) bool {
	return passThroughEnabled && upstreamAPIType == constant.APITypeAnthropic
}
