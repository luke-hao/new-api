package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAffiliateTopupRebatePercent(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "0", want: "0"},
		{input: "5", want: "5"},
		{input: "5.25", want: "5.25"},
		{input: "100", want: "100"},
		{input: "-0.01", wantErr: true},
		{input: "100.01", wantErr: true},
		{input: "5.251", wantErr: true},
		{input: "invalid", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeAffiliateTopupRebatePercent(test.input)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
