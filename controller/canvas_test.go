package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestCanvasGroupType(t *testing.T) {
	tests := map[string]string{
		"生图分组-flux": "image",
		"视频生成":      "video",
		"grok":      "text",
		"default":   "text",
	}
	for group, want := range tests {
		if got := canvasGroupType(group); got != want {
			t.Fatalf("canvasGroupType(%q) = %q, want %q", group, got, want)
		}
	}
}

func TestCreateCanvasSSOTicket(t *testing.T) {
	t.Setenv("CANVAS_SSO_ISSUER", "new-api-test")
	t.Setenv("CANVAS_SSO_AUDIENCE", "infinite-canvas-test")
	secret := "0123456789abcdef0123456789abcdef"
	now := time.Unix(1_800_000_000, 0).UTC()
	user := &model.User{Id: 42, Username: "canvas-user", DisplayName: "Canvas User", Role: common.RoleRootUser}

	ticket, expiresAt, err := createCanvasSSOTicket(user, now, secret)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := expiresAt, now.Add(canvasSSOTicketTTL); !got.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", got, want)
	}

	claims := &canvasSSOClaims{}
	parsed, err := jwt.ParseWithClaims(ticket, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithAudience("infinite-canvas-test"), jwt.WithIssuer("new-api-test"), jwt.WithTimeFunc(func() time.Time { return now.Add(time.Second) }))
	if err != nil || !parsed.Valid {
		t.Fatalf("ticket parse failed: %v", err)
	}
	if claims.Subject != strconv.Itoa(user.Id) || claims.Username != user.Username || claims.DisplayName != user.DisplayName || !claims.IsRoot || claims.ID == "" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil || claims.NotBefore == nil {
		t.Fatalf("registered times missing: %+v", claims.RegisteredClaims)
	}
}

func TestCreateCanvasSSOTicketRejectsShortSecret(t *testing.T) {
	_, _, err := createCanvasSSOTicket(&model.User{Id: 1}, time.Now(), "short")
	if err == nil {
		t.Fatal("expected short secret error")
	}
}

func TestGetCanvasCapabilitiesIncludesVideoModelCapabilities(t *testing.T) {
	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedGroupRatios := ratio_setting.GroupRatio2JSONString()
	savedAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatios))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(savedAutoGroups))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"视频生成":"Video"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"视频生成":1}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 1302, Username: "canvas-video-capability-user", Password: "password", Group: "default", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 204, Type: constant.ChannelTypeDoubaoVideo, Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "视频生成", Model: "happyhorse-1.1-t2v-720p", ChannelId: 204, Enabled: true}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/canvas/capabilities", nil)
	ctx.Set("id", 1302)

	GetCanvasCapabilities(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Groups            []canvasModelGroup               `json:"groups"`
			VideoCapabilities []playgroundVideoGroupCapability `json:"video_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.Groups, 1)
	require.Len(t, response.Data.VideoCapabilities, 1)
	require.Equal(t, "视频生成", response.Data.VideoCapabilities[0].Group)
	require.Len(t, response.Data.VideoCapabilities[0].Models, 1)
	capability := response.Data.VideoCapabilities[0].Models[0]
	require.Equal(t, "happyhorse-1.1-t2v-720p", capability.Model)
	require.Equal(t, []string{playgroundVideoModeText}, capability.Modes)
	require.Equal(t, makeIntRange(4, 15), capability.Parameters.Durations)
	require.Equal(t, []string{"720p"}, capability.Parameters.Resolutions)
	require.Equal(t, 9, capability.Parameters.MaxImageReferences)
}
