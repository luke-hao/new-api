package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResponsesFailoverChannelExclusion(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "database", true: "cache"}[cached], func(t *testing.T) {
			clearChannelGroupRoutingTables(t)
			insertRoutingTestChannel(t, 11001, 10, 0, "failover-a")
			insertRoutingTestChannel(t, 11002, 10, 0, "failover-a")
			insertRoutingTestChannel(t, 11003, 5, 0, "failover-a")
			insertRoutingTestChannel(t, 11004, 99, 0, "failover-b")
			common.MemoryCacheEnabled = cached
			if cached {
				InitChannelCache()
			}
			excluded := map[int]bool{11001: true}
			ch, err := GetChannelExcluding("failover-a", "routing-test-model", excluded)
			require.NoError(t, err)
			require.NotNil(t, ch)
			require.Equal(t, 11002, ch.Id)
			excluded[11002] = true
			ch, err = GetChannelExcluding("failover-a", "routing-test-model", excluded)
			require.NoError(t, err)
			require.NotNil(t, ch)
			require.Equal(t, 11003, ch.Id)
			excluded[11003] = true
			ch, err = GetChannelExcluding("failover-a", "routing-test-model", excluded)
			require.NoError(t, err)
			require.Nil(t, ch)
		})
	}
}
