package common

import (
	"encoding/json"
	"io"
	"strings"

	appcommon "github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

// Reasoning metadata describes the configuration sent upstream, never inferred
// model defaults or the amount of reasoning actually performed by the provider.
const ReasoningRelayInfoContextKey = "reasoning_relay_info"

const (
	ReasoningSpecified     = "specified"
	ReasoningUnspecified   = "unspecified"
	ReasoningAutomatic     = "automatic"
	ReasoningEnabled       = "enabled"
	ReasoningDisabled      = "disabled"
	ReasoningNotApplicable = "not_applicable"
	ReasoningUnknown       = "unknown"
)

type reasoningOptions struct {
	Effort    *string `json:"effort"`
	Enabled   *bool   `json:"enabled"`
	MaxTokens *int    `json:"max_tokens"`
}

type reasoningThinking struct {
	Type         string `json:"type"`
	BudgetTokens *int   `json:"budget_tokens"`
}

type reasoningGeminiConfig struct {
	ThinkingLevel       *string `json:"thinkingLevel"`
	ThinkingLevelSnake  *string `json:"thinking_level"`
	ThinkingBudget      *int    `json:"thinkingBudget"`
	ThinkingBudgetSnake *int    `json:"thinking_budget"`
}

type reasoningGeneration struct {
	ThinkingConfig      *reasoningGeminiConfig `json:"thinkingConfig"`
	ThinkingConfigSnake *reasoningGeminiConfig `json:"thinking_config"`
}

// Only decode configuration fields. Conversation content and media are skipped.
type reasoningEnvelope struct {
	Effort       *string           `json:"reasoning_effort"`
	Reasoning    *reasoningOptions `json:"reasoning"`
	OutputConfig *struct {
		Effort *string `json:"effort"`
	} `json:"output_config"`
	Thinking           json.RawMessage `json:"thinking"`
	Think              json.RawMessage `json:"think"`
	EnableThinking     *bool           `json:"enable_thinking"`
	ChatTemplateKwargs *struct {
		EnableThinking *bool `json:"enable_thinking"`
	} `json:"chat_template_kwargs"`
	GenerationConfig      *reasoningGeneration `json:"generationConfig"`
	GenerationConfigSnake *reasoningGeneration `json:"generation_config"`
}

func (info *RelayInfo) ResetReasoning() {
	if info == nil {
		return
	}
	info.ReasoningEffort = ""
	info.ThinkingBudgetTokens = nil
	info.ReasoningStatus = ReasoningUnknown
}

// CaptureReasoningStorage observes the exact passthrough bytes. Bytes preserves
// the storage's read position; errors affect metadata only, not forwarding.
func CaptureReasoningStorage(info *RelayInfo, storage appcommon.BodyStorage) {
	info.ResetReasoning()
	if info == nil || storage == nil {
		return
	}
	position, err := storage.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	defer func() {
		if _, restoreErr := storage.Seek(position, io.SeekStart); restoreErr != nil {
			info.ResetReasoning()
		}
	}()
	data, err := storage.Bytes()
	if err != nil {
		return
	}
	CaptureReasoningJSON(info, data)
}

// CaptureReasoningJSON must run after conversion, filtering and overrides, at
// the last point where the final upstream JSON is already available.
func CaptureReasoningJSON(info *RelayInfo, data []byte) {
	if info == nil {
		return
	}
	info.ResetReasoning()
	var fields *reasoningEnvelope
	if err := appcommon.Unmarshal(data, &fields); err != nil || fields == nil {
		return
	}
	result := reasoningResult{}
	if fields.Effort != nil {
		result.effort = strings.TrimSpace(*fields.Effort)
	}
	if fields.Reasoning != nil {
		if fields.Reasoning.Effort != nil {
			result.effort = strings.TrimSpace(*fields.Reasoning.Effort)
		}
		result.setEnabled(fields.Reasoning.Enabled)
		result.setBudget(fields.Reasoning.MaxTokens, false)
	}
	if fields.OutputConfig != nil && fields.OutputConfig.Effort != nil {
		result.effort = strings.TrimSpace(*fields.OutputConfig.Effort)
	}
	result.readThinking(fields.Thinking, false)
	result.readThinking(fields.Think, true)
	result.setEnabled(fields.EnableThinking)
	if fields.ChatTemplateKwargs != nil {
		result.setEnabled(fields.ChatTemplateKwargs.EnableThinking)
	}
	gen := fields.GenerationConfig
	if fields.GenerationConfigSnake != nil {
		gen = fields.GenerationConfigSnake
	}
	if gen != nil {
		config := gen.ThinkingConfig
		if gen.ThinkingConfigSnake != nil {
			config = gen.ThinkingConfigSnake
		}
		if config != nil {
			level := config.ThinkingLevel
			if config.ThinkingLevelSnake != nil {
				level = config.ThinkingLevelSnake
			}
			if level != nil {
				result.effort = strings.TrimSpace(*level)
			}
			budget := config.ThinkingBudget
			if config.ThinkingBudgetSnake != nil {
				budget = config.ThinkingBudgetSnake
			}
			result.setBudget(budget, true)
		}
	}
	if strings.EqualFold(result.effort, "none") {
		result.disabled = true
	}
	info.ReasoningEffort = result.effort
	info.ThinkingBudgetTokens = result.budget
	switch {
	case result.invalid:
		info.ReasoningStatus = ReasoningUnknown
	case result.disabled:
		info.ReasoningStatus = ReasoningDisabled
	case result.effort != "":
		info.ReasoningStatus = ReasoningSpecified
	case result.automatic:
		info.ReasoningStatus = ReasoningAutomatic
	case result.enabled || result.budget != nil:
		info.ReasoningStatus = ReasoningEnabled
	default:
		info.ReasoningStatus = ReasoningUnspecified
	}
}

type reasoningResult struct {
	effort                                string
	budget                                *int
	disabled, enabled, automatic, invalid bool
}

func (r *reasoningResult) setEnabled(value *bool) {
	if value == nil {
		return
	}
	if *value {
		r.enabled = true
	} else {
		r.disabled = true
	}
}

func (r *reasoningResult) setBudget(value *int, dynamic bool) {
	if value == nil {
		return
	}
	r.budget = value
	switch {
	case *value == 0:
		r.disabled = true
	case *value == -1 && dynamic:
		r.automatic = true
	case *value < 0:
		r.invalid = true
	default:
		r.enabled = true
	}
}

func (r *reasoningResult) readThinking(raw json.RawMessage, allowEffort bool) {
	if len(raw) == 0 || appcommon.GetJsonType(raw) == "null" {
		return
	}
	switch appcommon.GetJsonType(raw) {
	case "boolean":
		var enabled bool
		if appcommon.Unmarshal(raw, &enabled) != nil {
			r.invalid = true
			return
		}
		r.setEnabled(&enabled)
	case "string":
		var effort string
		if !allowEffort || appcommon.Unmarshal(raw, &effort) != nil {
			r.invalid = true
			return
		}
		r.effort = strings.TrimSpace(effort)
	case "object":
		var thinking reasoningThinking
		if appcommon.Unmarshal(raw, &thinking) != nil {
			r.invalid = true
			return
		}
		switch strings.ToLower(strings.TrimSpace(thinking.Type)) {
		case "":
		case "enabled":
			r.enabled = true
		case "disabled":
			r.disabled = true
		case "adaptive", "auto":
			r.automatic = true
		default:
			r.invalid = true
		}
		r.setBudget(thinking.BudgetTokens, false)
	default:
		r.invalid = true
	}
}

func (info *RelayInfo) AppendReasoningInfo(other map[string]interface{}) {
	if info == nil || other == nil {
		return
	}
	status := info.ReasoningStatus
	path := strings.SplitN(info.RequestURLPath, "?", 2)[0]
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings, relayconstant.RelayModeRerank,
		relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits,
		relayconstant.RelayModeModerations, relayconstant.RelayModeAudioSpeech,
		relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation,
		relayconstant.RelayModeVideoSubmit, relayconstant.RelayModeVideoFetchByID:
		status = ReasoningNotApplicable
	}
	if strings.HasSuffix(path, ":embedContent") || strings.HasSuffix(path, ":batchEmbedContents") {
		status = ReasoningNotApplicable
	}
	if status == "" {
		status = ReasoningUnknown
	}
	other["reasoning_status"] = status
	if status == ReasoningNotApplicable {
		return
	}
	if info.ReasoningEffort != "" {
		other["reasoning_effort"] = info.ReasoningEffort
	}
	if info.ThinkingBudgetTokens != nil {
		other["thinking_budget_tokens"] = *info.ThinkingBudgetTokens
	}
}
