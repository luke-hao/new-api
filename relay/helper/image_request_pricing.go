package helper

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
	"strings"
)

func imageRequestPricingMeta(info *relaycommon.RelayInfo, meta *types.TokenCountMeta) *types.TokenCountMeta {
	if meta == nil {
		meta = &types.TokenCountMeta{}
	}
	if !model_setting.IsGeminiModelSupportImagine(info.OriginModelName) {
		return meta
	}
	copy := *meta
	size := ""
	switch req := info.Request.(type) {
	case *dto.GeminiChatRequest:
		size = gjson.GetBytes(req.GenerationConfig.ImageConfig, "imageSize").String()
		if size == "" {
			size = gjson.GetBytes(req.GenerationConfig.ImageConfig, "image_size").String()
		}
	case *dto.GeneralOpenAIRequest:
		size = gjson.GetBytes(req.ExtraBody, "google.image_config.image_size").String()
		if size == "" {
			size = req.Size
		}
	}
	name := strings.ToLower(info.OriginModelName)
	if strings.HasSuffix(name, "-4k") {
		size = "4K"
	} else if strings.HasSuffix(name, "-2k") {
		size = "2K"
	}
	copy.ImagePriceTier, _ = dto.GPTImage2SizePriceTier(size)
	return &copy
}
