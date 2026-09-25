package service

import (
	"sync"
	"testing"
	"time"
)

func TestTokenActivityLifecycle(t *testing.T) {
	tracker := NewTokenActivityTracker()
	done := tracker.Begin(7)
	if m := tracker.Snapshot([]int{7, 8}); m[7].Active != 1 || m[7].RPM != 1 || m[8].Active != 0 {
		t.Fatalf("bad snapshot: %+v", m)
	}
	done()
	done()
	if m := tracker.Snapshot([]int{7})[7]; m.Active != 0 || m.RPM != 1 {
		t.Fatalf("bad release: %+v", m)
	}
	tracker.mu.Lock()
	tracker.entries[7].Seconds = [60]activitySecond{{Second: time.Now().Unix() - 61, Count: 9}}
	tracker.mu.Unlock()
	if m := tracker.Snapshot([]int{7})[7]; m.RPM != 0 {
		t.Fatal(m)
	}
	if m := NewTokenActivityTracker().Snapshot([]int{7})[7]; m.Active != 0 || m.RPM != 0 {
		t.Fatal(m)
	}
}
func TestTokenActivityConcurrent(t *testing.T) {
	tracker := NewTokenActivityTracker()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); release := tracker.Begin(9); tracker.Snapshot([]int{9}); release() }()
	}
	wg.Wait()
	m := tracker.Snapshot([]int{9})[9]
	if m.Active != 0 || m.RPM != 100 {
		t.Fatal(m)
	}
}
func TestTokenDayBoundsShanghai(t *testing.T) {
	now := time.Date(2026, 9, 25, 16, 0, 1, 0, time.UTC)
	start, end := TokenDayBounds(now)
	if start != time.Date(2026, 9, 25, 16, 0, 0, 0, time.UTC).Unix() || end-start != 86400 {
		t.Fatalf("%d %d", start, end)
	}
}
