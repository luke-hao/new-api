package common

import (
	"bytes"
	"io"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestCaptureReasoningJSON(t *testing.T) {
	tests := []struct {
		name, body, status, effort string
		budget                     *int
	}{
		{"chat unknown model", `{"model":"future-model","reasoning_effort":"xhigh"}`, ReasoningSpecified, "xhigh", nil},
		{"responses", `{"reasoning":{"effort":"max"}}`, ReasoningSpecified, "max", nil},
		{"future effort", `{"reasoning":{"effort":"future-effort"}}`, ReasoningSpecified, "future-effort", nil},
		{"none", `{"reasoning_effort":"none"}`, ReasoningDisabled, "none", nil},
		{"claude effort", `{"output_config":{"effort":"high"},"thinking":{"type":"adaptive"}}`, ReasoningSpecified, "high", nil},
		{"claude adaptive", `{"thinking":{"type":"adaptive"}}`, ReasoningAutomatic, "", nil},
		{"claude budget", `{"thinking":{"type":"enabled","budget_tokens":8192}}`, ReasoningEnabled, "", appcommon.GetPointer(8192)},
		{"claude off retains effort", `{"output_config":{"effort":"high"},"thinking":{"type":"disabled"}}`, ReasoningDisabled, "high", nil},
		{"zero budget", `{"thinking":{"budget_tokens":0}}`, ReasoningDisabled, "", appcommon.GetPointer(0)},
		{"gemini level", `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"HIGH"}}}`, ReasoningSpecified, "HIGH", nil},
		{"gemini snake", `{"generation_config":{"thinking_config":{"thinking_level":"low","thinking_budget":1234}}}`, ReasoningSpecified, "low", appcommon.GetPointer(1234)},
		{"gemini dynamic", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":-1}}}`, ReasoningAutomatic, "", appcommon.GetPointer(-1)},
		{"gemini zero", `{"generationConfig":{"thinkingConfig":{"thinkingBudget":0}}}`, ReasoningDisabled, "", appcommon.GetPointer(0)},
		{"gemini thought display is not enablement", `{"generationConfig":{"thinkingConfig":{"includeThoughts":false}}}`, ReasoningUnspecified, "", nil},
		{"openrouter nested wins", `{"reasoning_effort":"low","reasoning":{"effort":"high"}}`, ReasoningSpecified, "high", nil},
		{"openrouter enabled", `{"reasoning":{"enabled":true}}`, ReasoningEnabled, "", nil},
		{"openrouter disabled", `{"reasoning":{"enabled":false,"effort":"high"}}`, ReasoningDisabled, "high", nil},
		{"openrouter budget", `{"reasoning":{"max_tokens":4096}}`, ReasoningEnabled, "", appcommon.GetPointer(4096)},
		{"qwen off", `{"enable_thinking":false}`, ReasoningDisabled, "", nil},
		{"template off", `{"chat_template_kwargs":{"enable_thinking":false}}`, ReasoningDisabled, "", nil},
		{"glm enabled", `{"thinking":{"type":"enabled"}}`, ReasoningEnabled, "", nil},
		{"thinking boolean", `{"thinking":true}`, ReasoningEnabled, "", nil},
		{"ollama boolean", `{"think":false}`, ReasoningDisabled, "", nil},
		{"ollama effort", `{"think":"medium"}`, ReasoningSpecified, "medium", nil},
		{"missing", `{"model":"future-high","messages":[{"role":"user","content":"reasoning_effort: high"}]}`, ReasoningUnspecified, "", nil},
		{"empty effort", `{"reasoning_effort":"  "}`, ReasoningUnspecified, "", nil},
		{"nulls", `{"reasoning":null,"thinking":null,"enable_thinking":null}`, ReasoningUnspecified, "", nil},
		{"malformed", `{`, ReasoningUnknown, "", nil},
		{"null body", `null`, ReasoningUnknown, "", nil},
		{"array body", `[]`, ReasoningUnknown, "", nil},
		{"wrong effort type", `{"reasoning_effort":42}`, ReasoningUnknown, "", nil},
		{"wrong budget type", `{"thinking":{"budget_tokens":"8192"}}`, ReasoningUnknown, "", nil},
		{"unknown thinking mode", `{"thinking":{"type":"future-mode"}}`, ReasoningUnknown, "", nil},
		{"invalid budget", `{"thinking":{"budget_tokens":-1}}`, ReasoningUnknown, "", appcommon.GetPointer(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &RelayInfo{ReasoningEffort: "stale", ReasoningStatus: ReasoningSpecified, ThinkingBudgetTokens: appcommon.GetPointer(99)}
			body := []byte(tt.body)
			before := append([]byte(nil), body...)
			CaptureReasoningJSON(info, body)
			require.Equal(t, before, body)
			require.Equal(t, tt.status, info.ReasoningStatus)
			require.Equal(t, tt.effort, info.ReasoningEffort)
			require.Equal(t, tt.budget, info.ThinkingBudgetTokens)
		})
	}
}

func TestCaptureReasoningPassthroughPreservesReaderAndBody(t *testing.T) {
	// Include a large media-like field before the metadata to catch prefix-only
	// extraction and accidental forwarding of a partially consumed body.
	body := append([]byte(`{"input":"`), bytes.Repeat([]byte("A"), 2<<20)...)
	body = append(body, []byte(`","reasoning":{"effort":"xhigh"}}`)...)
	storage, err := appcommon.CreateBodyStorage(body)
	require.NoError(t, err)
	defer storage.Close()
	_, err = storage.Seek(17, io.SeekStart)
	require.NoError(t, err)
	info := &RelayInfo{}
	CaptureReasoningStorage(info, storage)
	position, err := storage.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.EqualValues(t, 17, position)
	require.Equal(t, "xhigh", info.ReasoningEffort)
	_, err = storage.Seek(0, io.SeekStart)
	require.NoError(t, err)
	sent, err := io.ReadAll(appcommon.ReaderOnly(storage))
	require.NoError(t, err)
	require.Equal(t, body, sent)
	storage.Close()
	CaptureReasoningStorage(info, storage)
	require.Equal(t, ReasoningUnknown, info.ReasoningStatus)
	require.Empty(t, info.ReasoningEffort)
}

func TestCaptureReasoningAfterOverridesAndRetry(t *testing.T) {
	info := &RelayInfo{}
	body := []byte(`{"reasoning":{"effort":"low"}}`)
	CaptureReasoningJSON(info, body)
	changed, err := ApplyParamOverride(body, map[string]interface{}{"operations": []interface{}{map[string]interface{}{"path": "reasoning.effort", "mode": "set", "value": "high"}}}, nil)
	require.NoError(t, err)
	CaptureReasoningJSON(info, changed)
	require.Equal(t, "high", info.ReasoningEffort)
	// A new attempt with a deleted/missing field must not retain earlier metadata.
	deleted, err := ApplyParamOverride(changed, map[string]interface{}{"operations": []interface{}{map[string]interface{}{"path": "reasoning.effort", "mode": "delete"}}}, nil)
	require.NoError(t, err)
	CaptureReasoningJSON(info, deleted)
	require.Equal(t, ReasoningUnspecified, info.ReasoningStatus)
	require.Empty(t, info.ReasoningEffort)
	info.ResetReasoning()
	require.Equal(t, ReasoningUnknown, info.ReasoningStatus)
}

func TestAppendReasoningInfo(t *testing.T) {
	info := &RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}
	CaptureReasoningJSON(info, []byte(`{"thinking":{"budget_tokens":0}}`))
	other := map[string]interface{}{"cache_tokens": 100, "model_ratio": 2.0}
	info.AppendReasoningInfo(other)
	require.Equal(t, ReasoningDisabled, other["reasoning_status"])
	require.Equal(t, 0, other["thinking_budget_tokens"])
	require.Equal(t, 100, other["cache_tokens"])
	require.Equal(t, 2.0, other["model_ratio"])
	info.RelayMode = relayconstant.RelayModeImagesEdits
	image := map[string]interface{}{}
	info.AppendReasoningInfo(image)
	require.Equal(t, map[string]interface{}{"reasoning_status": ReasoningNotApplicable}, image)
}
