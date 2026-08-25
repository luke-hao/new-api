package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolvePlaygroundMappedModel(t *testing.T) {
	mapped, ok := resolvePlaygroundMappedModel("studio-video", `{"studio-video":"alias-video","alias-video":"veo-3.1-generate-preview"}`)
	require.True(t, ok)
	require.Equal(t, "veo-3.1-generate-preview", mapped)

	_, ok = resolvePlaygroundMappedModel("a", `{"a":"b","b":"a"}`)
	require.False(t, ok)
}

func TestCollectPlaygroundVideoModelsIntersectsDuplicateRoutes(t *testing.T) {
	abilities := []model.AbilityWithChannel{
		{
			Ability:             model.Ability{Model: "shared-video"},
			ChannelType:         constant.ChannelTypeKling,
			ChannelModelMapping: `{"shared-video":"kling-v2-master"}`,
		},
		{
			Ability:             model.Ability{Model: "shared-video"},
			ChannelType:         constant.ChannelTypeGemini,
			ChannelModelMapping: `{"shared-video":"veo-3.1-generate-preview"}`,
		},
	}

	models := collectPlaygroundVideoModels(abilities)
	require.Len(t, models, 1)
	require.Equal(t, "shared-video", models[0].Model)
	require.Equal(t, "mixed", models[0].Profile)
	require.Equal(t, []string{playgroundVideoModeText, playgroundVideoModeImage}, models[0].Modes)
	require.Equal(t, []int{5}, models[0].Parameters.Durations)
	require.Equal(t, []string{"16:9", "9:16"}, models[0].Parameters.AspectRatios)
	require.Empty(t, models[0].Parameters.Resolutions)
	require.False(t, models[0].Parameters.SupportsSeed)
	require.Equal(t, 1, models[0].Parameters.MaxInputReferences)
}

func TestGetPlaygroundVideoCapabilitiesFiltersChannelsAndBuildsAutoGroup(t *testing.T) {
	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedGroupRatios := ratio_setting.GroupRatio2JSONString()
	savedAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatios))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(savedAutoGroups))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"video-a":"A","video-b":"B","auto":"Auto"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"video-a":1,"video-b":1.5,"private":2}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["video-a","video-b"]`))

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1301,
		Username: "video-capability-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)

	klingMapping := `{"shared-video":"kling-v2-master"}`
	veoMapping := `{"shared-video":"veo-3.1-generate-preview"}`
	channels := []model.Channel{
		{Id: 201, Type: constant.ChannelTypeKling, Status: common.ChannelStatusEnabled, ModelMapping: &klingMapping},
		{Id: 202, Type: constant.ChannelTypeGemini, Status: common.ChannelStatusEnabled, ModelMapping: &veoMapping},
		{Id: 203, Type: constant.ChannelTypeKling, Status: common.ChannelStatusManuallyDisabled},
	}
	require.NoError(t, db.Create(&channels).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "video-a", Model: "shared-video", ChannelId: 201, Enabled: true},
		{Group: "video-a", Model: "shared-video", ChannelId: 202, Enabled: true},
		{Group: "video-b", Model: "vidu2.0", ChannelId: 201, Enabled: true},
		{Group: "private", Model: "private-video", ChannelId: 201, Enabled: true},
		{Group: "video-b", Model: "disabled-video", ChannelId: 203, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/playground/video-capabilities", nil)
	ctx.Set("id", 1301)

	GetPlaygroundVideoCapabilities(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool                             `json:"success"`
		Data    []playgroundVideoGroupCapability `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	groups := make(map[string]playgroundVideoGroupCapability, len(response.Data))
	for _, group := range response.Data {
		groups[group.Group] = group
	}
	require.Contains(t, groups, "video-a")
	require.Contains(t, groups, "video-b")
	require.Contains(t, groups, "auto")
	require.NotContains(t, groups, "private")
	require.Len(t, groups["video-a"].Models, 1)
	require.Equal(t, "shared-video", groups["video-a"].Models[0].Model)
	require.NotContains(t, groups["video-b"].Models, playgroundVideoModelCapability{Model: "disabled-video"})
	require.Len(t, groups["auto"].Models, 2)
}
