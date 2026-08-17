package ratio_setting

import "testing"

func TestCompletionRatioExplicitConfigOverridesFamilyDefault(t *testing.T) {
	original := completionRatioMap.ReadAll()
	completionRatioMap.Clear()
	t.Cleanup(func() {
		completionRatioMap.Clear()
		completionRatioMap.AddAll(original)
	})

	completionRatioMap.Set("gpt-5.6-sol", 6)

	if got := GetCompletionRatio("gpt-5.6-sol"); got != 6 {
		t.Fatalf("GetCompletionRatio() = %v, want explicit ratio 6", got)
	}
	if got := GetCompletionRatioInfo("gpt-5.6-sol"); got.Ratio != 6 || got.Locked {
		t.Fatalf("GetCompletionRatioInfo() = %+v, want ratio 6 and unlocked", got)
	}
}

func TestCompletionRatioUsesFamilyDefaultWithoutExplicitConfig(t *testing.T) {
	original := completionRatioMap.ReadAll()
	completionRatioMap.Clear()
	t.Cleanup(func() {
		completionRatioMap.Clear()
		completionRatioMap.AddAll(original)
	})

	if got := GetCompletionRatio("gpt-5.6-unconfigured"); got != 8 {
		t.Fatalf("GetCompletionRatio() = %v, want family default ratio 8", got)
	}
	if got := GetCompletionRatioInfo("gpt-5.6-unconfigured"); got.Ratio != 8 || !got.Locked {
		t.Fatalf("GetCompletionRatioInfo() = %+v, want ratio 8 and locked", got)
	}
}
