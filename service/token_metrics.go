package service

import (
	"context"
	"fmt"
	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/singleflight"
	"sort"
	"sync"
	"time"
	_ "time/tzdata"
)

var tokenMetricsLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}()

func TokenDayBounds(now time.Time) (int64, int64) {
	n := now.In(tokenMetricsLocation)
	start := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, tokenMetricsLocation)
	return start.Unix(), start.AddDate(0, 0, 1).Unix()
}

type tokenUsageCacheEntry struct {
	Rows    []model.TokenDailyUsage
	Updated int64
	Expires time.Time
}

var tokenUsageCache = struct {
	sync.Mutex
	Entries map[string]tokenUsageCacheEntry
}{Entries: make(map[string]tokenUsageCacheEntry)}
var tokenUsageFlight singleflight.Group

func CachedTokenDailyUsage(ctx context.Context, userID int, ids []int) ([]model.TokenDailyUsage, int64, error) {
	sorted := append([]int(nil), ids...)
	sort.Ints(sorted)
	start, end := TokenDayBounds(time.Now())
	key := fmt.Sprintf("%d:%d:%v", userID, start, sorted)
	lookup := func() (tokenUsageCacheEntry, bool) {
		tokenUsageCache.Lock()
		defer tokenUsageCache.Unlock()
		e, ok := tokenUsageCache.Entries[key]
		return e, ok && time.Now().Before(e.Expires)
	}
	if e, ok := lookup(); ok {
		return e.Rows, e.Updated, nil
	}
	ch := tokenUsageFlight.DoChan(key, func() (interface{}, error) {
		if e, ok := lookup(); ok {
			return e, nil
		}
		queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, err := model.GetTokenDailyUsage(queryCtx, userID, sorted, start, end)
		if err != nil {
			return nil, err
		}
		e := tokenUsageCacheEntry{Rows: rows, Updated: time.Now().Unix(), Expires: time.Now().Add(30 * time.Second)}
		tokenUsageCache.Lock()
		for k, v := range tokenUsageCache.Entries {
			if time.Now().After(v.Expires) {
				delete(tokenUsageCache.Entries, k)
			}
		}
		if len(tokenUsageCache.Entries) >= 512 {
			for k := range tokenUsageCache.Entries {
				delete(tokenUsageCache.Entries, k)
				break
			}
		}
		tokenUsageCache.Entries[key] = e
		tokenUsageCache.Unlock()
		return e, nil
	})
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	case result := <-ch:
		if result.Err != nil {
			return nil, 0, result.Err
		}
		e := result.Val.(tokenUsageCacheEntry)
		return e.Rows, e.Updated, nil
	}
}
