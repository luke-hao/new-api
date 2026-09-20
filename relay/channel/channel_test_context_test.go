package channel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelProbeCancellationClosesUpstream(t *testing.T) {
	service.InitHttpClient()
	for _, phase := range []string{"headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			started := make(chan struct{})
			closed := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if phase == "body" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				close(started)
				select {
				case <-r.Context().Done():
					close(closed)
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")).WithContext(parent)
			req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
			require.NoError(t, err)
			info := &relaycommon.RelayInfo{IsChannelTest: true, ChannelMeta: &relaycommon.ChannelMeta{}}
			done := make(chan error, 1)
			readingBody := make(chan struct{})
			go func() {
				resp, err := DoRequest(c, req, info)
				if err == nil {
					defer resp.Body.Close()
					close(readingBody)
					_, err = io.ReadAll(resp.Body)
				}
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream did not start")
			}
			if phase == "body" {
				select {
				case <-readingBody:
				case <-time.After(2 * time.Second):
					t.Fatal("probe did not receive upstream headers")
				}
			}
			cancel()
			select {
			case err := <-done:
				require.Error(t, err)
				if phase == "body" {
					require.True(t, errors.Is(err, context.Canceled), err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled probe is still holding an upstream request")
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("upstream connection remained open after probe cancellation")
			}
		})
	}
}
