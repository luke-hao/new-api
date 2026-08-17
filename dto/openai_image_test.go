package dto

import (
	"math"
	"testing"
)

func TestImageRequestGetTokenCountMetaGPTImage2SizePricing(t *testing.T) {
	const basePrice = 0.06
	tests := []struct {
		name      string
		size      string
		wantPrice float64
	}{
		{name: "empty defaults to 1K", size: "", wantPrice: 0.06},
		{name: "auto defaults to 1K", size: "auto", wantPrice: 0.06},
		{name: "square 1K", size: "1024x1024", wantPrice: 0.06},
		{name: "portrait 1K", size: "1024x1536", wantPrice: 0.06},
		{name: "aspect ratio defaults to 1K", size: "1x1", wantPrice: 0.06},
		{name: "square 2K", size: "2048x2048", wantPrice: 0.15},
		{name: "landscape 2K", size: "2560x1440", wantPrice: 0.15},
		{name: "portrait 2K", size: "1440x2560", wantPrice: 0.15},
		{name: "explicit 2K", size: "2K", wantPrice: 0.15},
		{name: "landscape 4K", size: "3840x2160", wantPrice: 0.20},
		{name: "portrait 4K", size: "2160x3840", wantPrice: 0.20},
		{name: "explicit 4K", size: "4K", wantPrice: 0.20},
		{name: "unknown defaults to 1K", size: "unknown", wantPrice: 0.06},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &ImageRequest{Model: "gpt-image-2", Size: tt.size}
			gotPrice := basePrice * request.GetTokenCountMeta().ImagePriceRatio
			if math.Abs(gotPrice-tt.wantPrice) > 1e-9 {
				t.Fatalf("price for size %q = %.12f, want %.12f", tt.size, gotPrice, tt.wantPrice)
			}
		})
	}
}

func TestIsGPTImage2SizeStrictlyAbove2K(t *testing.T) {
	tests := []struct {
		name string
		size string
		want bool
	}{
		{name: "empty", size: "", want: false},
		{name: "auto", size: "auto", want: false},
		{name: "explicit 1K", size: "1K", want: false},
		{name: "explicit 2K", size: "2K", want: false},
		{name: "2K boundary landscape", size: "2560x1440", want: false},
		{name: "2K boundary portrait", size: "1440x2560", want: false},
		{name: "just above 2K", size: "2561x1440", want: true},
		{name: "explicit 4K", size: "4K", want: true},
		{name: "explicit 4K case and whitespace", size: " 4k ", want: true},
		{name: "4K dimensions", size: "3840x2160", want: true},
		{name: "unknown label", size: "3K", want: false},
		{name: "invalid dimensions", size: "wide", want: false},
		{name: "non-positive dimensions", size: "0x3840", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGPTImage2SizeStrictlyAbove2K(tt.size); got != tt.want {
				t.Fatalf("IsGPTImage2SizeStrictlyAbove2K(%q) = %t, want %t", tt.size, got, tt.want)
			}
		})
	}
}

func TestGPTImage2SizePriceTier(t *testing.T) {
	tests := []struct {
		size     string
		wantTier string
		wantOK   bool
	}{
		{size: "", wantTier: "1K", wantOK: true},
		{size: "2048x2048", wantTier: "2K", wantOK: true},
		{size: "3840x2160", wantTier: "4K", wantOK: true},
		{size: "unknown", wantTier: "", wantOK: false},
	}

	for _, tt := range tests {
		tier, ok := GPTImage2SizePriceTier(tt.size)
		if tier != tt.wantTier || ok != tt.wantOK {
			t.Fatalf("GPTImage2SizePriceTier(%q) = (%q, %t), want (%q, %t)", tt.size, tier, ok, tt.wantTier, tt.wantOK)
		}
	}
}
