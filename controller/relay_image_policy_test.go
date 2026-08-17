package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestValidateNative4KImageRequest(t *testing.T) {
	tests := []struct {
		name    string
		group   string
		model   string
		size    string
		request dto.Request
		wantErr bool
	}{
		{name: "empty size rejected", group: native4KImageGroup, model: "gpt-image-2", size: "", wantErr: true},
		{name: "auto rejected", group: native4KImageGroup, model: "gpt-image-2", size: "auto", wantErr: true},
		{name: "1K rejected", group: native4KImageGroup, model: "gpt-image-2", size: "1K", wantErr: true},
		{name: "2K rejected", group: native4KImageGroup, model: "gpt-image-2", size: "2K", wantErr: true},
		{name: "2560 boundary rejected", group: native4KImageGroup, model: "gpt-image-2", size: "2560x1440", wantErr: true},
		{name: "unknown rejected", group: native4KImageGroup, model: "gpt-image-2", size: "3K", wantErr: true},
		{name: "2561 accepted", group: native4KImageGroup, model: "gpt-image-2", size: "2561x1440", wantErr: false},
		{name: "4K accepted", group: native4KImageGroup, model: "gpt-image-2", size: "4K", wantErr: false},
		{name: "other group unchanged", group: "生图分组-image2", model: "gpt-image-2", size: "1K", wantErr: false},
		{name: "other model unchanged", group: native4KImageGroup, model: "gpt-image-1", size: "1K", wantErr: false},
		{name: "non-image request rejected", group: native4KImageGroup, model: "gpt-image-2", request: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := tt.request
			if request == nil && tt.name != "non-image request rejected" {
				request = &dto.ImageRequest{Model: tt.model, Size: tt.size}
			}
			info := &relaycommon.RelayInfo{
				UsingGroup:      tt.group,
				OriginModelName: tt.model,
			}
			err := validateNative4KImageRequest(info, request)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if err != nil && err.Error() != native4KImageGroupSizeErrorMsg {
				t.Fatalf("error message = %q, want %q", err.Error(), native4KImageGroupSizeErrorMsg)
			}
		})
	}
}
