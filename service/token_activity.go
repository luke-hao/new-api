package service

import (
	"sync"
	"time"
)

type TokenActivity struct {
	Active int `json:"active"`
	RPM    int `json:"rpm"`
}
type activitySecond struct {
	Second int64
	Count  int
}
type activityEntry struct {
	Active  int
	Last    int64
	Seconds [60]activitySecond
}
type TokenActivityTracker struct {
	mu      sync.Mutex
	entries map[int]*activityEntry
	swept   int64
}

func NewTokenActivityTracker() *TokenActivityTracker {
	return &TokenActivityTracker{entries: make(map[int]*activityEntry)}
}

var LiveTokenActivity = NewTokenActivityTracker()

func (t *TokenActivityTracker) cleanup(now int64) {
	if now-t.swept < 60 {
		return
	}
	t.swept = now
	for id, e := range t.entries {
		if e.Active == 0 && now-e.Last >= 60 {
			delete(t.entries, id)
		}
	}
}
func (t *TokenActivityTracker) Begin(id int) func() {
	now := time.Now().Unix()
	t.mu.Lock()
	t.cleanup(now)
	e := t.entries[id]
	if e == nil {
		e = &activityEntry{}
		t.entries[id] = e
	}
	e.Active++
	e.Last = now
	slot := &e.Seconds[now%60]
	if slot.Second != now {
		*slot = activitySecond{Second: now}
	}
	slot.Count++
	t.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { t.mu.Lock(); e.Active--; e.Last = time.Now().Unix(); t.mu.Unlock() }) }
}
func (t *TokenActivityTracker) Snapshot(ids []int) map[int]TokenActivity {
	now := time.Now().Unix()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanup(now)
	out := make(map[int]TokenActivity, len(ids))
	for _, id := range ids {
		m := TokenActivity{}
		if e := t.entries[id]; e != nil {
			m.Active = e.Active
			for _, s := range e.Seconds {
				if now-s.Second < 60 && s.Second <= now {
					m.RPM += s.Count
				}
			}
		}
		out[id] = m
	}
	return out
}
