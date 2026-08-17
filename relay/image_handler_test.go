package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestShouldUseRawImageBody(t *testing.T) {
	t.Run("non pass-through keeps conversion", func(t *testing.T) {
		if shouldUseRawImageBody(false, &dto.ImageRequest{Model: "gpt-image-2"}) {
			t.Fatal("expected gpt-image-2 to use converted body when pass-through is disabled")
		}
	})

	t.Run("gpt image models bypass raw pass-through", func(t *testing.T) {
		if shouldUseRawImageBody(true, &dto.ImageRequest{Model: "gpt-image-2"}) {
			t.Fatal("expected gpt-image-2 to use converted body when pass-through is enabled")
		}
	})

	t.Run("other image models still use raw pass-through", func(t *testing.T) {
		if !shouldUseRawImageBody(true, &dto.ImageRequest{Model: "dall-e-3"}) {
			t.Fatal("expected non gpt-image models to keep raw pass-through")
		}
	})
}
