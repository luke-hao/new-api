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

func TestVideoModesForModelExcludesImageOnlyModels(t *testing.T) {
	for _, modelName := range []string{
		"gpt-image-2",
		"nano-banana-2",
		"gemini-3-pro-image-2k",
	} {
		t.Run(modelName, func(t *testing.T) {
			require.Empty(t, videoModesForModel(constant.ChannelTypeGemini, modelName))
			require.Empty(t, videoModesForModel(constant.ChannelTypeSora, modelName))
		})
	}

	require.Equal(t, []string{playgroundVideoModeText, playgroundVideoModeImage}, videoModesForModel(constant.ChannelTypeSora, "sora-2"))
}

func TestPlaygroundVideoModelProfilesMatchStudioControls(t *testing.T) {
	sd25, ok := getPlaygroundVideoModelCapability(constant.ChannelTypeDoubaoVideo, "sd-2.5-720p不卡脸")
	require.True(t, ok)
	require.Equal(t, fullVideoModes(), sd25.Modes)
	require.Equal(t, makeIntRange(4, 30), sd25.Parameters.Durations)
	require.Equal(t, []string{"720p"}, sd25.Parameters.Resolutions)
	require.Equal(t, 30, sd25.Parameters.MaxImageReferences)
	require.Equal(t, 10, sd25.Parameters.MaxVideoReferences)
	require.Equal(t, 10, sd25.Parameters.MaxAudioReferences)

	h3, ok := getPlaygroundVideoModelCapability(constant.ChannelTypeMiniMax, "official-h3-1080p")
	require.True(t, ok)
	require.Equal(t, []string{playgroundVideoModeReference}, h3.Modes)
	require.Equal(t, makeIntRange(5, 15), h3.Parameters.Durations)
	require.Equal(t, []string{"1080p"}, h3.Parameters.Resolutions)
	require.Equal(t, 9, h3.Parameters.MaxImageReferences)
	require.Zero(t, h3.Parameters.MaxVideoReferences)
	require.Equal(t, 3, h3.Parameters.MaxAudioReferences)

	wang, ok := getPlaygroundVideoModelCapability(constant.ChannelTypeDoubaoVideo, "wang-3.0-480p")
	require.True(t, ok)
	require.Equal(t, makeIntRange(4, 30), wang.Parameters.Durations)
	require.Equal(t, []string{"480p"}, wang.Parameters.Resolutions)
	require.Equal(t, 10, wang.Parameters.MaxImageReferences)
	require.Zero(t, wang.Parameters.MaxVideoReferences)
	require.Zero(t, wang.Parameters.MaxAudioReferences)
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
	require.Equal(t, 1, models[0].Parameters.MaxImageReferences)
	require.Zero(t, models[0].Parameters.MaxVideoReferences)
	require.Zero(t, models[0].Parameters.MaxAudioReferences)
}

func TestGetPlaygroundVideoCapabilitiesUsesFixedVideoGroup(t *testing.T) {
	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedGroupRatios := ratio_setting.GroupRatio2JSONString()
	savedAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatios))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(savedAutoGroups))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"视频生成":"Video","private":"Private"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"视频生成":1,"private":2}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))

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
		{Group: "视频生成", Model: "shared-video", ChannelId: 201, Enabled: true},
		{Group: "视频生成", Model: "shared-video", ChannelId: 202, Enabled: true},
		{Group: "视频生成", Model: "vidu2.0", ChannelId: 201, Enabled: true},
		{Group: "视频生成", Model: "nano-banana-2", ChannelId: 202, Enabled: true},
		{Group: "private", Model: "private-video", ChannelId: 201, Enabled: true},
		{Group: "视频生成", Model: "disabled-video", ChannelId: 203, Enabled: true},
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
	require.Contains(t, groups, "视频生成")
	require.NotContains(t, groups, "private")
	require.Len(t, groups["视频生成"].Models, 2)
	require.Equal(t, "shared-video", groups["视频生成"].Models[0].Model)
	require.NotContains(t, groups["视频生成"].Models, playgroundVideoModelCapability{Model: "disabled-video"})
	for _, capability := range groups["视频生成"].Models {
		require.NotEqual(t, "nano-banana-2", capability.Model)
	}
}
