package controller

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func modelProbeFixture(t *testing.T) (model.ChannelGroupStabilityPolicy, []*model.Channel) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 9100, Username: "model-fixture", Role: common.RoleRootUser, Status: 1, Group: "probe"}).Error)
	channels := []*model.Channel{}
	for i := 1; i <= 4; i++ {
		priority := int64(5 - i)
		c := &model.Channel{Id: 9100 + i, Type: 1, Key: "fixture", Name: "fixture", Status: 1, Group: "probe", Models: "sol,astra", Priority: &priority}
		require.NoError(t, c.Insert())
		channels = append(channels, c)
	}
	parent, err := model.SaveChannelGroupStabilityConfig(model.ChannelGroupStabilityConfig{Group: "probe", Enabled: true, IntervalMinutes: 5, HealthyThresholdSeconds: 8, ProbeTimeoutSeconds: 20}, time.Now().UnixMilli())
	require.NoError(t, err)
	child, err := model.GetChannelModelPolicy("probe", "sol")
	require.NoError(t, err)
	return model.ResolveChannelModelPolicy(*parent, *child), channels
}

func TestModelStabilityExecutionScopesProbesAndHealthyShortCircuit(t *testing.T) {
	policy, channels := modelProbeFixture(t)
	durations := map[int]int64{9101: 10000, 9102: 12000, 9103: 9000, 9104: 9500}
	calls := []int{}
	probe := func(ctx context.Context, c *model.Channel, user, timeout int, names ...string) channelStabilityProbeResult {
		require.Equal(t, []string{"sol"}, names)
		calls = append(calls, c.Id)
		return channelStabilityProbeResult{channel: c, success: true, latencyMs: durations[c.Id]}
	}
	many := func(ctx context.Context, cs []*model.Channel, user, timeout int, names ...string) map[int]channelStabilityProbeResult {
		results := map[int]channelStabilityProbeResult{}
		for _, c := range cs {
			results[c.Id] = probe(ctx, c, user, timeout, names...)
		}
		return results
	}
	runChannelModelStabilityWithProbe(context.Background(), policy, true, true, probe, many)
	require.Len(t, calls, 4)
	require.NoError(t, model.PopulateEffectiveModelRoutings(channels, "probe", "sol"))
	require.Equal(t, int64(4), *channels[0].EffectivePriority)
	require.Equal(t, int64(1), *channels[1].EffectivePriority)
	require.Equal(t, int64(3), *channels[2].EffectivePriority)
	require.Equal(t, int64(2), *channels[3].EffectivePriority)
	require.Equal(t, int64(9000), channels[2].ModelResponseTime)
	require.Zero(t, channels[2].ResponseTime)
	require.NoError(t, model.PopulateEffectiveModelRoutings(channels, "probe", "astra"))
	require.Equal(t, int64(3), *channels[1].EffectivePriority)
	require.Zero(t, channels[2].ModelResponseTime)
	child, err := model.GetChannelModelPolicy("probe", "sol")
	require.NoError(t, err)
	require.True(t, child.Initialized)
	parent, err := model.GetOrCreateChannelGroupStabilityPolicy("probe")
	require.NoError(t, err)
	policy = model.ResolveChannelModelPolicy(*parent, *child)
	calls = nil
	durations[9101] = 8000
	runChannelModelStabilityWithProbe(context.Background(), policy, true, false, probe, many)
	require.Equal(t, []int{9101}, calls)
	child, err = model.GetChannelModelPolicy("probe", "sol")
	require.NoError(t, err)
	require.Equal(t, "healthy", child.LastResult)
	policy = model.ResolveChannelModelPolicy(*parent, *child)
	calls = nil
	durations[9101] = 10000
	runChannelModelStabilityWithProbe(context.Background(), policy, true, false, probe, many)
	require.Len(t, calls, 4)
	require.Equal(t, 9101, calls[0])
}

func TestModelStabilityAPIAndPagination(t *testing.T) {
	_, channels := modelProbeFixture(t)
	db := model.DB
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	router := gin.New()
	router.GET("/channels", GetAllChannels)
	router.GET("/search", SearchChannels)
	router.GET("/stability", GetChannelGroupStability)
	router.PUT("/stability", UpdateChannelGroupStability)
	router.PUT("/routing", UpdateChannelGroupRouting)
	call := func(method, path, body string) map[string]interface{} {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var result map[string]interface{}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		require.Equal(t, true, result["success"], w.Body.String())
		return result
	}
	call("PUT", "/routing", `{"group":"probe","model":"astra","updates":[{"channel_id":9104,"priority":99}]}`)
	for _, path := range []string{"/channels?group=probe&routing_model=astra&p=1&page_size=1", "/search?group=probe&routing_model=astra&keyword=fixture&p=1&page_size=1"} {
		r := call("GET", path, "")
		d := r["data"].(map[string]interface{})
		items := d["items"].([]interface{})
		require.Len(t, items, 1)
		require.Equal(t, float64(channels[3].Id), items[0].(map[string]interface{})["id"])
		require.Equal(t, float64(99), items[0].(map[string]interface{})["effective_priority"])
	}
	r := call("GET", "/stability?group=probe", "")
	require.Len(t, r["data"].(map[string]interface{})["models"], 2)
	call("PUT", "/stability", `{"group":"probe","model":"astra","paused":true,"interval_minutes_override":2,"healthy_threshold_seconds_override":null,"probe_timeout_seconds_override":null}`)
	r = call("GET", "/stability?group=probe&model=astra", "")
	d := r["data"].(map[string]interface{})
	require.Equal(t, true, d["paused"])
	require.Equal(t, false, d["enabled"])
	require.Equal(t, float64(2), d["interval_minutes"])
	require.Equal(t, float64(8), d["healthy_threshold_seconds"])
	call("PUT", "/stability", `{"group":"probe","model":"astra","paused":false,"interval_minutes_override":null}`)
	r = call("GET", "/stability?group=probe&model=astra", "")
	d = r["data"].(map[string]interface{})
	require.Equal(t, float64(5), d["interval_minutes"])
}
