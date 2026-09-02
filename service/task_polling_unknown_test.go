package service

import "testing"

func TestIsTransientUnknownVideoStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "unknown", body: `{"status":"unknown"}`, want: true},
		{name: "unknown case and whitespace", body: `{"status":" UNKNOWN "}`, want: true},
		{name: "completed", body: `{"status":"completed"}`, want: false},
		{name: "invalid json", body: `not-json`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientUnknownVideoStatus([]byte(tt.body)); got != tt.want {
				t.Fatalf("isTransientUnknownVideoStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
