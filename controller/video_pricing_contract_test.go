package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestPricingVideoContractsFillMissingMetadata(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.Channel{Id: 151, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Vendor{Id: 15, Name: "sd-官方区", Icon: "Doubao.Color"}).Error)
	var fixture struct {
		Data []struct {
			Name string `json:"model_name"`
			Unit string `json:"price_unit"`
		}
	}
	data, err := os.ReadFile("../common/testdata/aicopy_pricing_20260913.json")
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(data, &fixture))
	for _, row := range fixture.Data {
		require.NoError(t, db.Create(&model.Ability{Group: "视频生成", Model: row.Name, ChannelId: 151, Enabled: true}).Error)
	}
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)
	pricing := pricingByModelName(model.GetPricing())
	for _, row := range fixture.Data {
		p, ok := pricing[row.Name]
		require.True(t, ok, row.Name)
		unit := row.Unit
		if unit == "" {
			unit = "次"
		}
		require.Equal(t, unit, p.PriceUnit, row.Name)
		require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, p.SupportedEndpointTypes, row.Name)
	}
	missing := pricing["【官方稳定版】sd2.0-480p-满血"]
	require.Equal(t, "秒", missing.PriceUnit)
	require.Equal(t, "Doubao.Color", missing.Icon)
	require.Equal(t, 15, missing.VendorID)
	require.Contains(t, missing.Description, "15秒")
	require.Equal(t, "/v1/videos", model.GetSupportedEndpointMap()["openai-video"].Path)
}
