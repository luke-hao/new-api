package migrationgate

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func until(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatal("condition deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStreamDrainAndQueuedHandoff(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		close(started)
		<-finish
		io.WriteString(w, "data: last\n\n")
	}))
	defer old.Close()
	var newCalls atomic.Int32
	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { newCalls.Add(1); io.WriteString(w, "new") }))
	defer newServer.Close()
	g, err := New(old.URL, 10, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	g.Resume()
	front := httptest.NewServer(g)
	defer front.Close()
	response, err := http.Get(front.URL)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	g.Pause()
	if err := g.Switch(newServer.URL); err == nil {
		t.Fatal("switched during active SSE")
	}
	queued := make(chan string, 1)
	go func() {
		r, e := http.Get(front.URL)
		if e != nil {
			queued <- e.Error()
			return
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		queued <- string(b)
	}()
	until(t, func() bool { return g.Status().Waiting == 1 })
	if newCalls.Load() != 0 {
		t.Fatal("queued request escaped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if g.WaitDrained(ctx) == nil {
		t.Fatal("declared drained before SSE completed")
	}
	close(finish)
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !strings.Contains(string(body), "last") {
		t.Fatal("stream truncated")
	}
	if err := g.WaitDrained(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := g.Switch(newServer.URL); err != nil {
		t.Fatal(err)
	}
	g.Resume()
	if got := <-queued; got != "new" {
		t.Fatal(got)
	}
	if newCalls.Load() != 1 {
		t.Fatal("duplicate or missing request")
	}
}

func TestCancellationQueueBoundsAndFailureResume(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, "old") }))
	defer upstream.Close()
	g, _ := New(upstream.URL, 1, 100*time.Millisecond)
	front := httptest.NewServer(g)
	defer front.Close()
	ctx, cancel := context.WithCancel(context.Background())
	request, _ := http.NewRequestWithContext(ctx, "GET", front.URL, nil)
	done := make(chan struct{})
	go func() {
		r, _ := http.DefaultClient.Do(request)
		if r != nil {
			r.Body.Close()
		}
		close(done)
	}()
	until(t, func() bool { return g.Status().Waiting == 1 })
	r, err := http.Get(front.URL)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 503 {
		t.Fatal("queue limit not enforced")
	}
	cancel()
	<-done
	until(t, func() bool { return g.Status().Waiting == 0 })
	if calls.Load() != 0 {
		t.Fatal("cancelled request executed")
	}
	r, err = http.Get(front.URL)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 503 {
		t.Fatal("queue deadline not enforced")
	}
	// Abort a cutover by reopening the original backend without replaying cancelled work.
	g.Resume()
	r, err = http.Get(front.URL)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if calls.Load() != 1 || r.StatusCode != 200 {
		t.Fatal("resume failed")
	}
}

func TestStartupClosedAndTargetValidation(t *testing.T) {
	for _, target := range []string{"https://example.com", "http://10.0.0.1:3000", "http://user@127.0.0.1:3000"} {
		if _, err := New(target, 1, time.Second); err == nil {
			t.Fatal(target)
		}
	}
	g, _ := New("http://127.0.0.1:3000", 1, time.Second)
	if !g.Status().Paused {
		t.Fatal("restart must require explicit admission")
	}
	g.Resume()
	if g.Switch("http://127.0.0.1:3001") == nil {
		t.Fatal("switch allowed without pause")
	}
}
