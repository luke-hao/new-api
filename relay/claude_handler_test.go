package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

func TestEnsureClaudeAdaptiveThinkingDisplay(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		thinking    *dto.Thinking
		wantDisplay string
	}{
		{
			name:        "opus 4.8 defaults to summarized",
			model:       "claude-opus-4-8",
			thinking:    &dto.Thinking{Type: "adaptive"},
			wantDisplay: "summarized",
		},
		{
			name:        "opus 4.7 defaults to summarized",
			model:       "claude-opus-4-7",
			thinking:    &dto.Thinking{Type: "adaptive"},
			wantDisplay: "summarized",
		},
		{
			name:        "explicit omitted is preserved",
			model:       "claude-opus-4-8",
			thinking:    &dto.Thinking{Type: "adaptive", Display: "omitted"},
			wantDisplay: "omitted",
		},
		{
			name:        "enabled thinking is unchanged",
			model:       "claude-opus-4-8",
			thinking:    &dto.Thinking{Type: "enabled"},
			wantDisplay: "",
		},
		{
			name:        "opus 4.6 adaptive default is unchanged",
			model:       "claude-opus-4-6",
			thinking:    &dto.Thinking{Type: "adaptive"},
			wantDisplay: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.ClaudeRequest{Model: tt.model, Thinking: tt.thinking}
			ensureClaudeAdaptiveThinkingDisplay(request)
			if request.Thinking.Display != tt.wantDisplay {
				t.Fatalf("display = %q, want %q", request.Thinking.Display, tt.wantDisplay)
			}
		})
	}
}

func TestShouldUseRawClaudeBody(t *testing.T) {
	tests := []struct {
		name               string
		passThroughEnabled bool
		upstreamAPIType    int
		want               bool
	}{
		{
			name:               "disabled never passes through",
			passThroughEnabled: false,
			upstreamAPIType:    constant.APITypeAnthropic,
			want:               false,
		},
		{
			name:               "anthropic upstream keeps native body",
			passThroughEnabled: true,
			upstreamAPIType:    constant.APITypeAnthropic,
			want:               true,
		},
		{
			name:               "openai upstream requires conversion",
			passThroughEnabled: true,
			upstreamAPIType:    constant.APITypeOpenAI,
			want:               false,
		},
		{
			name:               "codex upstream requires conversion",
			passThroughEnabled: true,
			upstreamAPIType:    constant.APITypeCodex,
			want:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseRawClaudeBody(tt.passThroughEnabled, tt.upstreamAPIType); got != tt.want {
				t.Fatalf("shouldUseRawClaudeBody(%v, %d) = %v, want %v", tt.passThroughEnabled, tt.upstreamAPIType, got, tt.want)
			}
		})
	}
}
