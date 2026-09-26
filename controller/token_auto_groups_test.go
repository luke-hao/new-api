package controller

import (
	"context"
	"errors"
	"github.com/QuantumNous/new-api/types"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/postgres"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPersonalAutoTest(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	savedGroups := setting.UserUsableGroups2JSONString()
	savedRatio := ratio_setting.GroupRatio2JSONString()
	savedSpecial := ratio_setting.GroupGroupRatio2JSONString()
	savedAuto := setting.AutoGroups2JsonString()
	memory, retries := common.MemoryCacheEnabled, common.RetryTimes
	common.MemoryCacheEnabled = false
	common.RetryTimes = 1
	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(savedGroups)
		_ = ratio_setting.UpdateGroupRatioByJSONString(savedRatio)
		_ = ratio_setting.UpdateGroupGroupRatioByJSONString(savedSpecial)
		_ = setting.UpdateAutoGroupsByJsonString(savedAuto)
		common.MemoryCacheEnabled, common.RetryTimes = memory, retries
		model.InvalidatePricingCache()
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString("{\"a\":\"Affordable\",\"b\":\"Backup\",\"image\":\"Images\"}"))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString("{\"a\":0.1,\"b\":0.5,\"image\":1,\"private\":2}"))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString("{\"default\":{\"b\":0.3}}"))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString("[\"a\"]"))
	for index, group := range []string{"a", "b", "image", "private"} {
		channel := model.Channel{Id: index + 1, Type: constant.ChannelTypeOpenAI, Name: group, Key: "fixture", Status: common.ChannelStatusEnabled, Group: group, Models: "gpt-4o", BaseURL: common.GetPointer("http://127.0.0.1:1")}
		require.NoError(t, db.Create(&channel).Error)
		names := []string{"gpt-4o", "gpt-image-2", "nano-banana-pro"}
		if group == "b" {
			names = append(names, "claude-sonnet-4")
		}
		if group == "image" {
			names = []string{"gpt-image-2", "nano-banana-pro"}
		}
		for _, name := range names {
			require.NoError(t, db.Create(&model.Ability{Group: group, Model: name, ChannelId: channel.Id, Enabled: true, Priority: common.GetPointer(int64(1)), Weight: 1}).Error)
		}
	}
	model.InvalidatePricingCache()
	model.GetPricing()
	return db
}

func personalAutoContext(t *testing.T, groups []string, retry bool, path string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", path, nil)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto")
	encoded, _ := common.Marshal(groups)
	require.NoError(t, middleware.SetupContextForToken(c, &model.Token{Id: 1, UserId: 1, Group: "auto", AutoGroups: model.TokenAutoGroups(encoded), CrossGroupRetry: retry}))
	return c
}

func TestPersonalAutoOrderingFallbackAndBilling(t *testing.T) {
	setupPersonalAutoTest(t)
	c := personalAutoContext(t, []string{"a", "b"}, true, "/v1/responses")
	p := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"}
	first, group, err := service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "a", group)
	require.Equal(t, 1, first.Id)
	c.Set("use_channel", []string{"1"})
	p.IncreaseRetry()
	next, group, err := service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "b", group)
	require.Equal(t, 2, next.Id)
	info := &relaycommon.RelayInfo{UserGroup: "default", UsingGroup: "auto"}
	price := helper.HandleGroupRatio(c, info)
	require.Equal(t, "b", info.UsingGroup)
	require.Equal(t, 0.3, price.GroupRatio)
	c.Set("use_channel", []string{"1", "2"})
	p.IncreaseRetry()
	exhausted, _, err := service.CacheGetRandomSatisfiedChannel(p)
	require.Nil(t, exhausted)
	require.Error(t, err)
	c2 := personalAutoContext(t, []string{"b", "a"}, true, "/v1/responses")
	picked, group, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c2, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.NoError(t, err)
	require.Equal(t, "b", group)
	require.Equal(t, 2, picked.Id)
}

func TestPersonalAutoRetryOffAndMissingModel(t *testing.T) {
	setupPersonalAutoTest(t)
	c := personalAutoContext(t, []string{"a", "b"}, false, "/v1/responses")
	p := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"}
	_, _, err := service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	c.Set("use_channel", []string{"1"})
	p.IncreaseRetry()
	ch, _, err := service.CacheGetRandomSatisfiedChannel(p)
	require.Nil(t, ch)
	require.Error(t, err)
	c = personalAutoContext(t, []string{"a", "b"}, false, "/v1/responses")
	_, group, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "claude-sonnet-4"})
	require.NoError(t, err)
	require.Equal(t, "b", group)
}

func TestPersonalAutoPermissionsModelsAndAffinity(t *testing.T) {
	db := setupPersonalAutoTest(t)
	c := personalAutoContext(t, []string{"a", "b"}, true, "/v1/responses")
	service.SetTokenAutoPreferredChannel(c, 2)
	picked, group, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.NoError(t, err)
	require.Equal(t, "a", group)
	require.Equal(t, 1, picked.Id)
	c.Set("channel_affinity_skip_retry_on_failure", true)
	require.False(t, service.ShouldSkipRetryAfterChannelAffinityFailure(c))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString("{\"b\":\"Backup\"}"))
	c = personalAutoContext(t, []string{"a", "b"}, true, "/v1/responses")
	_, group, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.NoError(t, err)
	require.Equal(t, "b", group)
	c = personalAutoContext(t, []string{"a"}, true, "/v1/responses")
	_, _, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.Error(t, err)
	for _, name := range []string{"gpt-image-2", "nano-banana-pro"} {
		c = personalAutoContext(t, []string{"b"}, true, "/v1/chat/completions")
		if name == "nano-banana-pro" {
			require.NoError(t, service.PrepareTokenAutoRoute(c, name))
		} else {
			require.Error(t, service.PrepareTokenAutoRoute(c, name))
		}
	}
	c = personalAutoContext(t, []string{"b"}, true, "/v1/images/generations")
	require.Error(t, service.PrepareTokenAutoRoute(c, "gpt-4o"))
	require.NoError(t, service.PrepareTokenAutoRoute(c, "gpt-image-2"))
	c = personalAutoContext(t, []string{"b"}, true, "/v1/images/edits")
	require.NoError(t, service.PrepareTokenAutoRoute(c, "gpt-image-2"))
	c = personalAutoContext(t, []string{"b"}, true, "/v1/messages/count_tokens")
	require.NoError(t, service.PrepareTokenAutoRoute(c, "claude-sonnet-4"))
	require.NoError(t, db.Delete(&model.Ability{}, "channel_id = ?", 2).Error)
	c = personalAutoContext(t, []string{"b"}, true, "/v1/responses")
	_, _, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.Error(t, err)
}

func TestPersonalAutoCRUDCompatibilityAndBatch(t *testing.T) {
	db := setupPersonalAutoTest(t)
	token := seedToken(t, db, 1, "ordered", "ordered-fixture")
	for _, groups := range [][]string{{}, {"a", "a"}, {"private"}} {
		ctx, rec := newAuthenticatedContext(t, http.MethodPut, "/api/token/batch/group", map[string]any{"ids": []int{token.Id}, "group": "auto", "auto_groups": groups}, 1)
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
		UpdateTokenGroupBatch(ctx)
		require.False(t, decodeAPIResponse(t, rec).Success)
	}
	foreign := seedToken(t, db, 2, "foreign", "foreign-fixture")
	ctx, rec := newAuthenticatedContext(t, http.MethodPut, "/api/token/batch/group", map[string]any{"ids": []int{token.Id, foreign.Id}, "group": "auto", "auto_groups": []string{"b", "a"}, "cross_group_retry": true}, 1)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	UpdateTokenGroupBatch(ctx)
	require.True(t, decodeAPIResponse(t, rec).Success)
	var got model.Token
	require.NoError(t, db.First(&got, token.Id).Error)
	require.Equal(t, []string{"b", "a"}, got.AutoGroups.Groups())
	var other model.Token
	require.NoError(t, db.First(&other, foreign.Id).Error)
	require.Empty(t, other.AutoGroups.Groups())
	body := map[string]any{"id": token.Id, "name": "renamed", "group": "auto", "unlimited_quota": true, "expired_time": -1, "cross_group_retry": true}
	ctx, rec = newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)
	require.True(t, decodeAPIResponse(t, rec).Success)
	require.NoError(t, db.First(&got, token.Id).Error)
	require.Equal(t, []string{"b", "a"}, got.AutoGroups.Groups())
	ctx, rec = newAuthenticatedContext(t, http.MethodPut, "/api/token/batch/group", map[string]any{"ids": []int{token.Id}, "group": "a"}, 1)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	UpdateTokenGroupBatch(ctx)
	require.True(t, decodeAPIResponse(t, rec).Success)
	require.NoError(t, db.First(&got, token.Id).Error)
	require.Equal(t, []string{"b", "a"}, got.AutoGroups.Groups())
	ctx, rec = newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)
	require.Contains(t, rec.Body.String(), "\"auto_groups\":[\"b\",\"a\"]")
}

func TestPersonalAutoModelListIntersectionAndCandidates(t *testing.T) {
	setupPersonalAutoTest(t)
	savedSelfUse := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() { operation_setting.SelfUseModeEnabled = savedSelfUse })
	c := personalAutoContext(t, []string{"a"}, true, "/v1/models")
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-4o": true, "claude-sonnet-4": true, "gpt-image-2": true})
	// No user lookup is needed for self-use policy in this fixture.
	c.Set("id", 0)
	rec := httptest.NewRecorder()
	// Use a new context so the recorder is directly inspectable.
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("GET", "/v1/models", nil)
	ctx.Keys = c.Keys
	ListModels(ctx, constant.ChannelTypeOpenAI)
	require.Equal(t, map[string]struct{}{"gpt-4o": {}, "gpt-image-2": {}}, decodeListModelsResponse(t, rec))
	eligible, err := service.GetUserTextAutoGroups("default")
	require.NoError(t, err)
	require.True(t, eligible["a"])
	require.True(t, eligible["b"])
	require.False(t, eligible["image"])
	require.False(t, eligible["private"])
	capabilities, err := service.GetUserAutoGroupCapabilities("default")
	require.NoError(t, err)
	require.True(t, capabilities["image"].Image)
	require.False(t, capabilities["image"].Text)
	require.True(t, capabilities["a"].Text)
	require.True(t, capabilities["a"].Image)
}

func TestPersonalAutoImageOnlyGroupOrderingAndBilling(t *testing.T) {
	setupPersonalAutoTest(t)
	imageOnly := model.TokenAutoGroups(`["image"]`)
	require.NoError(t, service.ValidateTokenAutoGroups("default", imageOnly))
	c := personalAutoContext(t, []string{"image", "a"}, true, "/v1/images/generations")
	p := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-image-2"}
	first, group, err := service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "image", group)
	require.Equal(t, 3, first.Id)
	info := &relaycommon.RelayInfo{UserGroup: "default", UsingGroup: "auto"}
	require.Equal(t, 1.0, helper.HandleGroupRatio(c, info).GroupRatio)
	p.IncreaseRetry()
	_, group, err = service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "image", group)
	p.IncreaseRetry()
	next, group, err := service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "a", group)
	require.Equal(t, 1, next.Id)
	info = &relaycommon.RelayInfo{UserGroup: "default", UsingGroup: "auto"}
	require.Equal(t, 0.1, helper.HandleGroupRatio(c, info).GroupRatio)

	c = personalAutoContext(t, []string{"image", "a"}, true, "/v1/chat/completions")
	_, group, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.NoError(t, err)
	require.Equal(t, "a", group)
	imageList := personalAutoContext(t, []string{"image"}, true, "/v1/models")
	common.SetContextKey(imageList, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(imageList, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-4o": true, "gpt-image-2": true})
	imageList.Set("id", 0)
	previousSelfUse := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() { operation_setting.SelfUseModeEnabled = previousSelfUse })
	listRecorder := httptest.NewRecorder()
	listContext, _ := gin.CreateTestContext(listRecorder)
	listContext.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	listContext.Keys = imageList.Keys
	ListModels(listContext, constant.ChannelTypeOpenAI)
	require.Equal(t, map[string]struct{}{"gpt-image-2": {}}, decodeListModelsResponse(t, listRecorder))
	c = personalAutoContext(t, []string{"image"}, true, "/v1/videos")
	require.Error(t, service.PrepareTokenAutoRoute(c, "gpt-image-2"))
}

func TestPersonalAutoGroupOnlyImagePriceModelList(t *testing.T) {
	setupPersonalAutoTest(t)
	previousSelfUse := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() { operation_setting.SelfUseModeEnabled = previousSelfUse })
	previousPrices := ratio_setting.ImageTokenGroupPrices2JSONString()
	previousGroups := ratio_setting.ImageTokenBillingGroups2JSONString()
	t.Cleanup(func() {
		_ = ratio_setting.UpdateImageTokenGroupPricesByJSONString(previousPrices)
		_ = ratio_setting.UpdateImageTokenBillingGroupsByJSONString(previousGroups)
	})
	require.NoError(t, ratio_setting.UpdateImageTokenBillingGroupsByJSONString(`["image"]`))
	require.NoError(t, ratio_setting.UpdateImageTokenGroupPricesByJSONString(`{"image":{"gpt-image-2":{"input":1,"output":1,"image_input":1,"image_output":1,"cached_input":1,"cached_image_input":1,"cache_creation":1}}}`))
	c := personalAutoContext(t, []string{"image"}, true, "/v1/models")
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-image-2": true})
	c.Set("id", 0)
	recorder := httptest.NewRecorder()
	list, _ := gin.CreateTestContext(recorder)
	list.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	list.Keys = c.Keys
	ListModels(list, constant.ChannelTypeOpenAI)
	require.Equal(t, map[string]struct{}{"gpt-image-2": {}}, decodeListModelsResponse(t, recorder))
}

func TestPersonalAutoCreateAndPermissionValidation(t *testing.T) {
	db := setupPersonalAutoTest(t)
	body := map[string]any{"name": "Created auto", "group": "auto", "auto_groups": []string{"a", "b"}, "unlimited_quota": true, "expired_time": -1, "cross_group_retry": true}
	c, rec := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	AddToken(c)
	require.True(t, decodeAPIResponse(t, rec).Success)
	var token model.Token
	require.NoError(t, db.Where("name = ?", "Created auto").First(&token).Error)
	require.Equal(t, []string{"a", "b"}, token.AutoGroups.Groups())
	body["auto_groups"] = []string{"private"}
	c, rec = newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	AddToken(c)
	require.False(t, decodeAPIResponse(t, rec).Success)
}

func TestPersonalAutoPostgresAndRedis(t *testing.T) {
	dsn := os.Getenv("TEST_AUTO_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_AUTO_POSTGRES_DSN is required")
	}
	previousDB := model.DB
	oldMaster := common.IsMasterNode
	oldSQLite, oldPostgres, oldMysql := common.UsingSQLite, common.UsingPostgreSQL, common.UsingMySQL
	common.IsMasterNode = false
	common.UsingSQLite = false
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	t.Setenv("SQL_DSN", dsn)
	require.NoError(t, model.InitDB())
	common.IsMasterNode = oldMaster
	t.Cleanup(func() {
		common.UsingSQLite, common.UsingPostgreSQL, common.UsingMySQL = oldSQLite, oldPostgres, oldMysql
	})
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Dedicated disposable database, never a production DSN.
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	oldDB, oldRedis, oldRDB := previousDB, common.RedisEnabled, common.RDB
	model.DB = db
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: os.Getenv("TEST_AUTO_REDIS_ADDR")})
	t.Cleanup(func() { _ = common.RDB.Close(); model.DB, common.RedisEnabled, common.RDB = oldDB, oldRedis, oldRDB })
	fixtureKey, err := common.GenerateKey()
	require.NoError(t, err)
	token := model.Token{UserId: 7, Key: fixtureKey, Name: "cache", Group: "auto", AutoGroups: model.TokenAutoGroups("[\"a\",\"b\"]"), CrossGroupRetry: true}
	require.NoError(t, token.Insert())
	token.AutoGroups = model.TokenAutoGroups("[\"b\",\"a\"]")
	require.NoError(t, token.Update())
	got, err := model.GetTokenByKey(token.Key, false)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, got.AutoGroups.Groups())
	var persisted model.Token
	require.NoError(t, db.First(&persisted, token.Id).Error)
	require.Equal(t, []string{"b", "a"}, persisted.AutoGroups.Groups())
	count, err := model.BatchUpdateTokenGroup([]int{token.Id}, 7, "a", false)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	exists, err := common.RDB.Exists(context.Background(), "token:"+common.GenerateHMAC(token.Key)).Result()
	require.NoError(t, err)
	require.Zero(t, exists, "batch update must synchronously invalidate the old cached order")
	require.NoError(t, db.First(&got, token.Id).Error)
	require.Equal(t, "a", got.Group)
	require.Equal(t, []string{"b", "a"}, got.AutoGroups.Groups())
}

func TestPersonalAutoPreselectionAndZeroRetry(t *testing.T) {
	setupPersonalAutoTest(t)
	common.RetryTimes = 0
	c := personalAutoContext(t, []string{"a", "b"}, true, "/v1/chat/completions")
	preview := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o", Preselect: true}
	_, group, err := service.CacheGetRandomSatisfiedChannel(preview)
	require.NoError(t, err)
	require.Equal(t, "a", group)
	p := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: "gpt-4o"}
	_, group, err = service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "a", group, "preselection must not consume a group attempt")
	failure := types.NewErrorWithStatusCode(errors.New("upstream failed"), types.ErrorCodeDoRequestFailed, 502)
	require.True(t, shouldRetry(c, failure, 0), "next group retains its first attempt")
	streamFailure := types.NewErrorWithStatusCode(errors.New("stream already exposed"), types.ErrorCodeIncompleteStream, 502, types.ErrOptionWithSkipRetry())
	require.False(t, shouldRetry(c, streamFailure, 0), "cross-group retry must preserve the stream replay guard")
	p.IncreaseRetry()
	_, group, err = service.CacheGetRandomSatisfiedChannel(p)
	require.NoError(t, err)
	require.Equal(t, "b", group)
	require.False(t, shouldRetry(c, failure, 0), "no retry after the final candidate")
}
