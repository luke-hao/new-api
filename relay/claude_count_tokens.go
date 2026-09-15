package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

// Reuse the native adaptor's authentication, proxy, header overrides and beta
// query handling without routing this operation through message generation.
type claudeCountTokensAdaptor struct{ claude.Adaptor }

func (a *claudeCountTokensAdaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	endpoint, err := a.Adaptor.GetRequestURL(info)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	parsed.Path += "/count_tokens"
	return parsed.String(), nil
}

func ClaudeCountTokens(c *gin.Context) *types.NewAPIError {
	badRequest := func(err error) *types.NewAPIError {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest)
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return badRequest(err)
	}
	raw, err := storage.Bytes()
	if err != nil {
		return badRequest(err)
	}
	var request struct {
		Model    string            `json:"model"`
		Messages []json.RawMessage `json:"messages"`
	}
	if err = common.Unmarshal(raw, &request); err != nil {
		return badRequest(err)
	}
	if strings.TrimSpace(request.Model) == "" || len(request.Messages) == 0 {
		return badRequest(errors.New("model and non-empty messages are required"))
	}
	info := relaycommon.GenRelayInfoClaude(c, nil)
	info.InitChannelMeta(c)
	if info.ApiType != constant.APITypeAnthropic {
		return types.NewErrorWithStatusCode(errors.New("count_tokens requires an Anthropic-compatible channel"), types.ErrorCodeInvalidApiType, http.StatusNotImplemented)
	}
	body, err := prepareClaudeCountTokensBody(c, info, raw)
	if err != nil {
		return badRequest(err)
	}
	response, err := channel.DoApiRequest(&claudeCountTokensAdaptor{}, c, info, bytes.NewReader(body))
	if err != nil {
		return types.NewErrorWithStatusCode(errors.New("upstream token count request failed"), types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(response)
	// Counts and error responses are small. Bound buffering even if an upstream
	// accidentally sends a generation stream or a large HTML error page.
	const responseLimit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil || len(data) > responseLimit {
		return types.NewErrorWithStatusCode(errors.New("invalid upstream token count response"), types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		var count struct {
			InputTokens *int64 `json:"input_tokens"`
		}
		if err := common.Unmarshal(data, &count); err != nil || count.InputTokens == nil || *count.InputTokens < 0 {
			return types.NewErrorWithStatusCode(errors.New("upstream response is missing a valid input_tokens count"), types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
		}
	}
	service.IOCopyBytesGracefully(c, response, data)
	return nil
}

func prepareClaudeCountTokensBody(c *gin.Context, info *relaycommon.RelayInfo, raw []byte) ([]byte, error) {
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return nil, err
	}
	// Match the native Messages passthrough option: admin body overrides and
	// model rewriting are disabled together when raw passthrough is enabled.
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		return raw, nil
	}
	body, err := sjson.SetBytes(raw, "model", info.UpstreamModelName)
	if err != nil {
		return nil, err
	}
	if prompt := info.ChannelSetting.SystemPrompt; prompt != "" {
		var fields map[string]json.RawMessage
		if err := common.Unmarshal(body, &fields); err != nil {
			return nil, err
		}
		system := fields["system"]
		var value any
		if len(system) == 0 || string(system) == "null" {
			value = prompt
		} else if info.ChannelSetting.SystemPromptOverride {
			var text string
			if common.Unmarshal(system, &text) == nil {
				value = prompt
				if strings.TrimSpace(text) != "" {
					value = prompt + "\n" + strings.TrimSpace(text)
				}
			} else {
				var blocks []json.RawMessage
				if err := common.Unmarshal(system, &blocks); err != nil {
					return nil, err
				}
				prefix, err := common.Marshal(map[string]string{"type": "text", "text": prompt})
				if err != nil {
					return nil, err
				}
				value = append([]json.RawMessage{prefix}, blocks...)
			}
		}
		if value != nil {
			encoded, err := common.Marshal(value)
			if err != nil {
				return nil, err
			}
			body, err = sjson.SetRawBytes(body, "system", encoded)
			if err != nil {
				return nil, err
			}
		}
	}
	body, err = relaycommon.RemoveDisabledFields(body, info.ChannelOtherSettings, false)
	if err != nil {
		return nil, err
	}
	if len(info.ParamOverride) > 0 {
		body, err = relaycommon.ApplyParamOverrideWithRelayInfo(body, info)
	}
	return body, err
}
