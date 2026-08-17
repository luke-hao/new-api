package common

import (
	"encoding/json"
	"regexp"
	"strings"
)

const (
	PublicPoolChannelUnavailableMessage = "当前号池渠道暂时无可用资源，请稍后重试"
	PublicPoolChannelUnavailableCode    = "pool_channel_unavailable"
)

var (
	publicErrorStatusPrefixPattern = regexp.MustCompile(`(?i)^\s*status_code\s*=\s*([0-9]{3})\s*,?\s*`)
	publicTopologyPatterns         = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bupstreams?(?:[_\s-]*(?:error|request|channel|provider|group|id))*\b`),
		regexp.MustCompile(`(?i)\bdistributor\b`),
		regexp.MustCompile(`(?i)\brelay(?:[_\s-]*(?:error|request|channel|provider|server))?\b`),
		regexp.MustCompile(`(?i)\bproxy(?:[_\s-]*(?:error|request|channel|provider|server))?\b`),
		regexp.MustCompile(`(?i)\bunder\s+group\b`),
		regexp.MustCompile(`(?i)\bno\s+available\s+channels?\b`),
		regexp.MustCompile(`(?i)\bfailed\s+to\s+get\s+available\s+channels?\b`),
		regexp.MustCompile(`(?i)\bchannels?\s*(?:#|id\s*[:=]?)\s*[0-9]+\b`),
		regexp.MustCompile(`(?i)\bchannel[_\s-]*(?:id|name|type)\b`),
		regexp.MustCompile(`(?i)\bbase[_\s-]*url\b`),
	}
)

func ContainsUpstreamTopologyDetail(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, pattern := range publicTopologyPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}

	lower := strings.ToLower(text)
	if strings.Contains(lower, "上游") || strings.Contains(lower, "中转") || strings.Contains(lower, "中轉") {
		return true
	}

	hasGroup := strings.Contains(lower, "分组") || strings.Contains(lower, "分組")
	hasChannel := strings.Contains(lower, "渠道") || strings.Contains(lower, "通道") || strings.Contains(lower, "管道")
	hasAvailabilityFailure := strings.Contains(lower, "无可用") || strings.Contains(lower, "無可用") ||
		strings.Contains(lower, "获取") || strings.Contains(lower, "獲取") ||
		strings.Contains(lower, "失败") || strings.Contains(lower, "失敗")
	return hasGroup && hasChannel && hasAvailabilityFailure
}

func SanitizePublicErrorText(text string) (string, bool) {
	if !ContainsUpstreamTopologyDetail(text) {
		return text, false
	}
	if match := publicErrorStatusPrefixPattern.FindStringSubmatch(text); len(match) == 2 {
		return "status_code=" + match[1] + ", " + PublicPoolChannelUnavailableMessage, true
	}
	return PublicPoolChannelUnavailableMessage, true
}

func SanitizePublicErrorPayload(data []byte) ([]byte, bool) {
	var payload any
	if len(data) == 0 || json.Unmarshal(data, &payload) != nil || !isPublicErrorEnvelope(payload) {
		return data, false
	}
	if !payloadContainsTopologyDetail(payload) {
		return data, false
	}

	sanitizePublicErrorEnvelope(payload)
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return data, false
	}
	return sanitized, true
}

func isPublicErrorEnvelope(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if errValue, exists := m["error"]; exists && errValue != nil {
		return true
	}
	typeValue := strings.ToLower(strings.TrimSpace(stringValue(m["type"])))
	if typeValue == "error" || typeValue == "upstream_error" || strings.HasSuffix(typeValue, ".failed") {
		return true
	}
	if response, ok := m["response"].(map[string]any); ok {
		if errValue, exists := response["error"]; exists && errValue != nil {
			return true
		}
		return strings.EqualFold(stringValue(response["status"]), "failed")
	}
	return false
}

func payloadContainsTopologyDetail(value any) bool {
	switch v := value.(type) {
	case string:
		return ContainsUpstreamTopologyDetail(v)
	case []any:
		for _, item := range v {
			if payloadContainsTopologyDetail(item) {
				return true
			}
		}
	case map[string]any:
		for key, item := range v {
			if ContainsUpstreamTopologyDetail(key) || payloadContainsTopologyDetail(item) {
				return true
			}
		}
	}
	return false
}

func sanitizePublicErrorEnvelope(value any) {
	m, ok := value.(map[string]any)
	if !ok {
		return
	}

	rootType := strings.ToLower(strings.TrimSpace(stringValue(m["type"])))
	if rootType == "upstream_error" {
		m["type"] = "error"
	}

	if errValue, exists := m["error"]; exists {
		switch errNode := errValue.(type) {
		case map[string]any:
			sanitizePublicErrorObject(errNode)
		case string:
			m["error"] = PublicPoolChannelUnavailableMessage
		default:
			m["error"] = map[string]any{
				"message": PublicPoolChannelUnavailableMessage,
				"type":    "new_api_error",
				"code":    PublicPoolChannelUnavailableCode,
			}
		}
		if _, nestedObject := errValue.(map[string]any); !nestedObject {
			m["code"] = PublicPoolChannelUnavailableCode
		}
	}

	if response, ok := m["response"].(map[string]any); ok {
		if errValue, exists := response["error"]; exists {
			switch errNode := errValue.(type) {
			case map[string]any:
				sanitizePublicErrorObject(errNode)
			case string:
				response["error"] = map[string]any{
					"message": PublicPoolChannelUnavailableMessage,
					"type":    "new_api_error",
					"code":    PublicPoolChannelUnavailableCode,
				}
			}
		}
		removePublicTopologyFields(response)
	}

	if _, hasMessage := m["message"]; hasMessage && m["error"] == nil {
		m["message"] = PublicPoolChannelUnavailableMessage
		m["type"] = "new_api_error"
		m["code"] = PublicPoolChannelUnavailableCode
	}
	removePublicTopologyFields(m)
}

func sanitizePublicErrorObject(m map[string]any) {
	m["message"] = PublicPoolChannelUnavailableMessage
	m["type"] = "new_api_error"
	m["code"] = PublicPoolChannelUnavailableCode
	if _, exists := m["detail"]; exists {
		m["detail"] = PublicPoolChannelUnavailableMessage
	}
	if _, exists := m["error_description"]; exists {
		m["error_description"] = PublicPoolChannelUnavailableMessage
	}
	removePublicTopologyFields(m)
	if nested, ok := m["error"].(map[string]any); ok {
		sanitizePublicErrorObject(nested)
	}
}

func removePublicTopologyFields(m map[string]any) {
	for _, key := range []string{
		"metadata", "upstream_request_id", "upstreamRequestId", "channel_id", "channelId",
		"channel_name", "channelName", "channel_type", "channelType", "base_url", "baseURL",
		"group", "distributor", "provider",
	} {
		delete(m, key)
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}
