package claude

import (
	"context"
	"errors"
	"github.com/QuantumNous/new-api/constant"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const interruptedStart = "event: message_start\r\n" + `data: {"type":"message_start","message":{"id":"msg_fixture","model":"claude-fixture","usage":{"input_tokens":366640,"output_tokens":2}}}` + "\r\n\r\n"

type failStreamReader struct{}

func (failStreamReader) Read([]byte) (int, error) {
	return 0, errors.New("fixture upstream read failed")
}

type cancelOnRead struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *cancelOnRead) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	if n > 0 {
		r.cancel()
	}
	return n, e
}

type failingStreamWriter struct{ header http.Header }

func (w *failingStreamWriter) Header() http.Header { return w.header }
func (w *failingStreamWriter) WriteHeader(int)     {}
func (w *failingStreamWriter) Write([]byte) (int, error) {
	return 0, errors.New("fixture downstream write failed")
}
func (w *failingStreamWriter) Flush() {}

func TestClaudeInterruptedPassthroughPreservesConfirmedUsage(t *testing.T) {
	for _, mode := range []string{"cancel_during_read", "downstream_write_failure", "upstream_read_failure", "incomplete_eof"} {
		t.Run(mode, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			var writer http.ResponseWriter = httptest.NewRecorder()
			if mode == "downstream_write_failure" {
				writer = &failingStreamWriter{header: make(http.Header)}
			}
			c, _ := gin.CreateTestContext(writer)
			reqCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(reqCtx)
			var body io.Reader = strings.NewReader(interruptedStart)
			if mode == "cancel_during_read" {
				body = &cancelOnRead{Reader: body, cancel: cancel}
			}
			if mode == "upstream_read_failure" {
				body = io.MultiReader(body, failStreamReader{})
			}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fixture"}, RelayFormat: types.RelayFormatClaude, IsStream: true}
			usage, apiErr := ClaudeStreamPassthroughHandler(c, &http.Response{StatusCode: 200, Body: io.NopCloser(body)}, info)
			t.Logf("scenario=%s usage=%+v error=%v", mode, usage, apiErr)
			require.NotNil(t, apiErr)
			require.NotNil(t, usage, "confirmed upstream usage must survive the transport failure")
			require.Equal(t, 366640, usage.PromptTokens)
			require.Equal(t, 2, usage.CompletionTokens)
			require.Equal(t, 366642, usage.TotalTokens)
			require.Equal(t, "anthropic", usage.UsageSemantic)
			require.True(t, types.IsSkipRetryError(apiErr))
		})
	}
}

func TestClaudeInterruptedPassthroughBeforeUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fixture"}, RelayFormat: types.RelayFormatClaude, IsStream: true}
	usage, apiErr := ClaudeStreamPassthroughHandler(c, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, info)
	require.NotNil(t, apiErr)
	require.Nil(t, usage)
}

type chunkedPassthroughReader struct {
	io.Reader
	size int
}

func (r *chunkedPassthroughReader) Read(p []byte) (int, error) {
	if len(p) > r.size {
		p = p[:r.size]
	}
	return r.Reader.Read(p)
}

type blockingPassthroughBody struct {
	io.Reader
	blocked   chan struct{}
	closed    chan struct{}
	once      sync.Once
	blockOnce sync.Once
}

func (b *blockingPassthroughBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	if n > 0 {
		return n, nil
	}
	if e != nil {
		b.blockOnce.Do(func() { close(b.blocked) })
		<-b.closed
		return 0, context.Canceled
	}
	return n, e
}
func (b *blockingPassthroughBody) Close() error { b.once.Do(func() { close(b.closed) }); return nil }

func TestClaudePassthroughCancellationUnblocksRead(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	body := &blockingPassthroughBody{Reader: strings.NewReader(interruptedStart), blocked: make(chan struct{}), closed: make(chan struct{})}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fixture"}, RelayFormat: types.RelayFormatClaude, IsStream: true}
	finished := make(chan struct{})
	go func() { <-body.blocked; cancel(); close(finished) }()
	usage, apiErr := ClaudeStreamPassthroughHandler(c, &http.Response{StatusCode: 200, Body: body}, info)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("cancel helper did not finish")
	}
	require.NotNil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 366640, usage.PromptTokens)
	require.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
}

func TestClaudePassthroughChunkedBytesAndConfirmedZeroOutput(t *testing.T) {
	raw := ": preserve-comment\r\n\r\n" + strings.Replace(interruptedStart, `"output_tokens":2`, `"output_tokens":0`, 1) +
		"event: message_stop\r\ndata: {\"type\":\"message_stop\"}\r\n\r\n"
	for _, size := range []int{1, 2, 7, 127, 4096} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fixture"}, RelayFormat: types.RelayFormatClaude, IsStream: true}
		usage, apiErr := ClaudeStreamPassthroughHandler(c, &http.Response{StatusCode: 200, Body: io.NopCloser(&chunkedPassthroughReader{Reader: strings.NewReader(raw), size: size})}, info)
		require.Nil(t, apiErr)
		require.Equal(t, raw, recorder.Body.String())
		require.Equal(t, 0, usage.CompletionTokens)
	}
}

func TestClaudePassthroughOversizeHasNoInventedUsage(t *testing.T) {
	old := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() { constant.StreamScannerMaxBufferMB = old })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fixture"}, RelayFormat: types.RelayFormatClaude, IsStream: true}
	raw := "data: " + strings.Repeat("x", (1<<20)+100)
	usage, apiErr := ClaudeStreamPassthroughHandler(c, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw))}, info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
}
