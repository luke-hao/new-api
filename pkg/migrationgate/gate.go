// Package migrationgate provides bounded HTTP admission and full response draining.
// It does not assert that background jobs or an application's database are drained.
package migrationgate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Status struct {
	Paused  bool
	Active  int
	Waiting int
	Target  string
}

type Gate struct {
	mu         sync.Mutex
	paused     bool
	active     int
	waiting    int
	changed    chan struct{}
	target     *url.URL
	maxWaiting int
	maxWait    time.Duration
}

func New(target string, maxWaiting int, maxWait time.Duration) (*Gate, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("target must be an HTTP loopback origin")
	}
	if maxWaiting < 1 || maxWait <= 0 {
		return nil, errors.New("positive queue bounds required")
	}
	// Restarting never implicitly sends queued traffic to a possibly stale writer.
	return &Gate{paused: true, changed: make(chan struct{}), target: u, maxWaiting: maxWaiting, maxWait: maxWait}, nil
}

func (g *Gate) signal() { close(g.changed); g.changed = make(chan struct{}) }

func (g *Gate) Status() Status {
	g.mu.Lock()
	defer g.mu.Unlock()
	return Status{g.paused, g.active, g.waiting, g.target.String()}
}

func (g *Gate) Pause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = true
	g.signal()
}

func (g *Gate) Resume() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = false
	g.signal()
}

func (g *Gate) WaitDrained(ctx context.Context) error {
	for {
		g.mu.Lock()
		if !g.paused {
			g.mu.Unlock()
			return errors.New("admission must remain paused")
		}
		if g.active == 0 {
			g.mu.Unlock()
			return nil
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Switch only changes the next request's destination. Callers must independently
// verify database write fencing, refunds, quota flushes and the replication watermark.
func (g *Gate) Switch(target string) error {
	validated, err := New(target, g.maxWaiting, g.maxWait)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused || g.active != 0 {
		return errors.New("pause and drain active responses first")
	}
	g.target = validated.target
	return nil
}

func (g *Gate) acquire(ctx context.Context) (*url.URL, error) {
	deadline := time.NewTimer(g.maxWait)
	defer deadline.Stop()
	g.mu.Lock()
	queued := false
	defer func() {
		if queued {
			g.waiting--
		}
		g.mu.Unlock()
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !g.paused {
			g.active++
			return g.target, nil
		}
		if !queued {
			if g.waiting >= g.maxWaiting {
				return nil, errors.New("queue full")
			}
			g.waiting++
			queued = true
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			g.mu.Lock()
			return nil, ctx.Err()
		case <-deadline.C:
			g.mu.Lock()
			return nil, errors.New("queue deadline")
		case <-changed:
			g.mu.Lock()
		}
	}
}

func (g *Gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, err := g.acquire(r.Context())
	if err != nil {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "Request admission temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func() { g.mu.Lock(); g.active--; g.signal(); g.mu.Unlock() }()
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	proxy.ServeHTTP(w, r)
}

// Control is exposed only on a permission-restricted Unix socket by the command.
func (g *Gate) Control(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/status" && r.Method == http.MethodGet {
		s := g.Status()
		fmt.Fprintf(w, "paused=%t\nactive=%d\nwaiting=%d\ntarget=%s\n", s.Paused, s.Active, s.Waiting, s.Target)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	switch r.URL.Path {
	case "/pause":
		g.Pause()
	case "/resume":
		g.Resume()
	case "/drain":
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := g.WaitDrained(ctx); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	case "/switch":
		if err := g.Switch(r.URL.Query().Get("target")); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	fmt.Fprintln(w, "ok")
}
