package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestGptImage2IsImageGenerationModel(t *testing.T) {
	for _, modelName := range []string{"gpt-image-2", "gpt-image-2-2026-04-21", "chatgpt-image-latest"} {
		if !IsImageGenerationModel(modelName) {
			t.Fatalf("expected %s to be an image generation model", modelName)
		}

		endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, modelName)
		if len(endpoints) == 0 || endpoints[0] != constant.EndpointTypeImageGeneration {
			t.Fatalf("expected %s to prefer image-generation endpoint, got %v", modelName, endpoints)
		}
	}
}
