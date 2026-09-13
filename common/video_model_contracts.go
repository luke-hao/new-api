package common

import (
	_ "embed"
	"net/url"
	"strings"
)

// VideoModelContract is the reviewed AICopy video catalog. Billing units come
// from /api/pricing; the documented public video protocol takes precedence over
// the legacy endpoint labels there. Prices and routing remain site settings.
type VideoModelContract struct {
	Model             string   `json:"model"`
	PriceUnit         string   `json:"price_unit"`
	Icon              string   `json:"icon"`
	Vendor            string   `json:"vendor"`
	Description       string   `json:"description"`
	UpstreamEndpoints []string `json:"upstream_endpoints"`
}

//go:embed video_model_contracts.json
var videoModelContractsJSON []byte

var videoModelContracts = func() map[string]VideoModelContract {
	var rows []VideoModelContract
	if err := Unmarshal(videoModelContractsJSON, &rows); err != nil {
		panic(err)
	}
	contracts := make(map[string]VideoModelContract, len(rows))
	for _, row := range rows {
		if row.Model == "" || (row.PriceUnit != "次" && row.PriceUnit != "秒") {
			panic("invalid video contract")
		}
		if _, exists := contracts[row.Model]; exists {
			panic("duplicate video contract")
		}
		contracts[row.Model] = row
	}
	return contracts
}()

func GetVideoModelContract(model string) (VideoModelContract, bool) {
	value, ok := videoModelContracts[model]
	return value, ok
}

func UsesAICopyVideoProtocol(baseURL, model string) bool {
	_, known := GetVideoModelContract(model)
	u, err := url.Parse(baseURL)
	return known && err == nil && strings.EqualFold(u.Hostname(), "api.aicopy.top")
}
