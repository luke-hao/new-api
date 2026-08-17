package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetPlaygroundImageModelCapability(t *testing.T) {
	tests := []struct {
		model     string
		profile   string
		protocol  string
		fixedSize string
		ok        bool
	}{
		{model: "gpt-image-2", profile: "gpt_image_2", protocol: "image_api", ok: true},
		{model: "nano-banana-2", profile: "gemini_image", protocol: "gemini_chat", ok: true},
		{model: "nano-banana-pro", profile: "gemini_image", protocol: "gemini_chat", ok: true},
		{model: "gemini-3-pro-image-2k", profile: "gemini_image", protocol: "gemini_chat", fixedSize: "2K", ok: true},
		{model: "gemini-3.1-flash-image-4k", profile: "gemini_image", protocol: "gemini_chat", fixedSize: "4K", ok: true},
		{model: "gemini-3.1-pro-preview", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			capability, ok := getPlaygroundImageModelCapability(tt.model)
			require.Equal(t, tt.ok, ok)
			if !tt.ok {
				return
			}
			require.Equal(t, tt.profile, capability.Profile)
			require.Equal(t, tt.protocol, capability.Protocol)
			require.Equal(t, tt.fixedSize, capability.FixedImageSize)
		})
	}
}

func TestGetPlaygroundImageCapabilitiesFiltersAndGroupsModels(t *testing.T) {
	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatios))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{
		"生图分组-image2":"image2",
		"生图分组-image2-4k(原生)":"image2 4k",
		"生图分组-nanobanana":"banana",
		"普通聊天分组":"chat"
	}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{
		"生图分组-image2":1,
		"生图分组-image2-4k(原生)":1.5,
		"生图分组-nanobanana":1,
		"普通聊天分组":1
	}`))

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1201,
		Username: "image-capability-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)

	bananaModels := []string{
		"gemini-3-pro-image-2k",
		"gemini-3-pro-image-4k",
		"gemini-3.1-flash-image-2k",
		"gemini-3.1-flash-image-4k",
		"nano-banana-2",
		"nano-banana-pro",
	}
	abilities := []model.Ability{
		{Group: "生图分组-image2", Model: "gpt-image-2", ChannelId: 1, Enabled: true},
		{Group: "生图分组-image2-4k(原生)", Model: "gpt-image-2", ChannelId: 2, Enabled: true},
		{Group: "普通聊天分组", Model: "gpt-5.6", ChannelId: 3, Enabled: true},
	}
	for index, modelName := range bananaModels {
		abilities = append(abilities, model.Ability{
			Group:     "生图分组-nanobanana",
			Model:     modelName,
			ChannelId: 10 + index,
			Enabled:   true,
		})
	}
	require.NoError(t, db.Create(&abilities).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/playground/image-capabilities", nil)
	ctx.Set("id", 1201)

	GetPlaygroundImageCapabilities(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool                             `json:"success"`
		Data    []playgroundImageGroupCapability `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 3)

	groups := make(map[string]playgroundImageGroupCapability, len(response.Data))
	for _, group := range response.Data {
		groups[group.Group] = group
	}
	require.Len(t, groups["生图分组-image2"].Models, 1)
	require.Len(t, groups["生图分组-image2-4k(原生)"].Models, 1)
	require.Len(t, groups["生图分组-nanobanana"].Models, 6)

	fixedSizes := map[string]string{}
	for _, capability := range groups["生图分组-nanobanana"].Models {
		fixedSizes[capability.Model] = capability.FixedImageSize
		require.Equal(t, "gemini_chat", capability.Protocol)
	}
	require.Equal(t, "2K", fixedSizes["gemini-3-pro-image-2k"])
	require.Equal(t, "4K", fixedSizes["gemini-3-pro-image-4k"])
	require.Empty(t, fixedSizes["nano-banana-2"])
}
