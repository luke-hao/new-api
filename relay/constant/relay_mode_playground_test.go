package constant

import "testing"

func TestPath2RelayModePlaygroundImages(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/pg/images/generations", want: RelayModeImagesGenerations},
		{path: "/pg/images/edits", want: RelayModeImagesEdits},
	}

	for _, tt := range tests {
		if got := Path2RelayMode(tt.path); got != tt.want {
			t.Fatalf("Path2RelayMode(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}
