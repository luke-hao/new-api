package service

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ResponsesStreamFailedKey = "responses_stream_failed"
const responsesChannelCooldown = time.Minute

type responsesFailureKey struct {
	Group, Model string
	Channel      int
}

var responsesFailures = struct {
	sync.Mutex
	until map[responsesFailureKey]time.Time
}{until: make(map[responsesFailureKey]time.Time)}

func isResponsesFailoverRequest(c *gin.Context) bool {
	return c != nil && c.Request != nil && strings.HasSuffix(strings.TrimSuffix(c.Request.URL.Path, "/"), "/responses")
}

func RecordResponsesChannelFailure(c *gin.Context, info *relaycommon.RelayInfo, err *types.NewAPIError) {
	if err == nil || !isResponsesFailoverRequest(c) {
		return
	}
	transient := err.GetErrorCode() == types.ErrorCodeIncompleteStream || err.StatusCode == 408 || err.StatusCode == 429 || err.StatusCode >= 500
	if !transient {
		return
	}
	c.Set(ResponsesStreamFailedKey, true)
	if c.Request.Context().Err() != nil || info == nil || info.ChannelMeta == nil {
		return
	}
	now := time.Now()
	responsesFailures.Lock()
	defer responsesFailures.Unlock()
	for k, until := range responsesFailures.until {
		if !now.Before(until) {
			delete(responsesFailures.until, k)
		}
	}
	if len(responsesFailures.until) < 4096 {
		responsesFailures.until[responsesFailureKey{info.UsingGroup, info.OriginModelName, info.ChannelId}] = now.Add(responsesChannelCooldown)
	}
}

func IsResponsesChannelCoolingDown(c *gin.Context, group, modelName string, channelID int) bool {
	if !isResponsesFailoverRequest(c) {
		return false
	}
	responsesFailures.Lock()
	defer responsesFailures.Unlock()
	return time.Now().Before(responsesFailures.until[responsesFailureKey{group, modelName, channelID}])
}

func selectResponsesFailoverChannel(param *RetryParam, group string, retry int) (*model.Channel, error) {
	if !isResponsesFailoverRequest(param.Ctx) {
		return model.GetRandomSatisfiedChannel(group, param.ModelName, retry)
	}
	used := make(map[int]bool)
	for _, value := range param.Ctx.GetStringSlice("use_channel") {
		if id, err := strconv.Atoi(value); err == nil {
			used[id] = true
		}
	}
	excluded := make(map[int]bool, len(used))
	for id := range used {
		excluded[id] = true
	}
	now := time.Now()
	responsesFailures.Lock()
	for k, until := range responsesFailures.until {
		if !now.Before(until) {
			delete(responsesFailures.until, k)
			continue
		}
		if k.Group == group && k.Model == param.ModelName {
			excluded[k.Channel] = true
		}
	}
	responsesFailures.Unlock()
	if len(excluded) == 0 {
		return model.GetRandomSatisfiedChannel(group, param.ModelName, retry)
	}
	channel, err := model.GetChannelExcluding(group, param.ModelName, excluded)
	if channel != nil || err != nil {
		return channel, err
	}
	// If every remaining channel is cooling down, prefer a last-resort attempt
	// over rejecting the request. Never revisit an attempt from this request.
	return model.GetChannelExcluding(group, param.ModelName, used)
}
