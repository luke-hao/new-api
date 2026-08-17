package model_setting

import "testing"

func TestIsGeminiModelSupportImagineAliases(t *testing.T) {
	imageModels := []string{
		"gemini-3-pro-image-2k",
		"gemini-3-pro-image-4k",
		"gemini-3.1-flash-image-2k",
		"gemini-3.1-flash-image-4k",
		"nano-banana-2",
		"nano-banana-pro",
	}
	for _, modelName := range imageModels {
		if !IsGeminiModelSupportImagine(modelName) {
			t.Fatalf("expected %q to support image generation", modelName)
		}
	}

	if IsGeminiModelSupportImagine("gemini-3.1-pro-preview") {
		t.Fatal("ordinary Gemini text model must not be classified as image generation")
	}
}
