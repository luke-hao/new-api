package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func countTestContext(body, endpoint string, settings dto.ChannelSettings) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens?beta=true", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("x-api-key", "client-secret")
	c.Request.Header.Set("anthropic-version", "2023-06-01")
	c.Request.Header.Set("anthropic-beta", "fixture-beta")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "fixture")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, endpoint)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "upstream-secret")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, settings)
	return c, recorder
}

func TestClaudeCountTokensForwardsNativeRequest(t *testing.T) {
	service.InitHttpClient()
	raw := `{"model":"fixture","messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}},{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}]}],"tools":[{"name":"lookup","input_schema":{"type":"object"},"vendor_extra":false}],"thinking":{"type":"enabled","budget_tokens":0},"vendor_extra":{"enabled":false,"large":9007199254740993}}`
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if string(body) != raw {
			t.Errorf("body changed: %s", body)
		}
		if r.URL.String() != "/v1/messages/count_tokens?beta=true" {
			t.Errorf("wrong URL: %s", r.URL)
		}
		if r.Header.Get("x-api-key") != "upstream-secret" || r.Header.Get("Authorization") != "" {
			t.Error("credentials were not isolated")
		}
		if r.Header.Get("anthropic-beta") != "fixture-beta" || r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("X-Fixture") != "override" {
			t.Error("missing headers")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_fixture")
		io.WriteString(w, `{"input_tokens":27,"vendor_extra":9007199254740993}`)
	}))
	defer upstream.Close()
	c, recorder := countTestContext(raw, upstream.URL, dto.ChannelSettings{PassThroughBodyEnabled: true})
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, map[string]interface{}{"X-Fixture": "override"})
	if err := ClaudeCountTokens(c); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || recorder.Code != 200 || recorder.Body.String() != `{"input_tokens":27,"vendor_extra":9007199254740993}` || recorder.Header().Get("request-id") != "req_fixture" {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body)
	}
}

func TestClaudeCountTokensUpstreamResponses(t *testing.T) {
	service.InitHttpClient()
	for _, test := range []struct {
		name      string
		status    int
		body      string
		wantError int
	}{
		{"zero", 200, `{"input_tokens":0}`, 0},
		{"missing", 200, `{}`, 502},
		{"null", 200, `{"input_tokens":null}`, 502},
		{"negative", 200, `{"input_tokens":-1}`, 502},
		{"fraction", 200, `{"input_tokens":2.5}`, 502},
		{"boolean", 200, `{"input_tokens":true}`, 502},
		{"HTML", 200, `<html>wrong endpoint</html>`, 502},
		{"not_supported", 404, `{"type":"error","error":{"type":"not_found_error","message":"unavailable"}}`, 0},
		{"limited", 429, `{"type":"error","error":{"type":"rate_limit_error","message":"retry later"}}`, 0},
		{"oversize", 200, strings.Repeat(" ", (1<<20)+1), 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "10")
				w.WriteHeader(test.status)
				io.WriteString(w, test.body)
			}))
			defer upstream.Close()
			c, recorder := countTestContext(`{"model":"fixture","messages":[{"role":"user","content":"hi"}]}`, upstream.URL, dto.ChannelSettings{PassThroughBodyEnabled: true})
			err := ClaudeCountTokens(c)
			if test.wantError != 0 {
				if err == nil || err.StatusCode != test.wantError {
					t.Fatalf("expected %d, got %v", test.wantError, err)
				}
			} else if err != nil || recorder.Code != test.status || recorder.Body.String() != test.body || recorder.Header().Get("Retry-After") != "10" {
				t.Fatalf("response changed: %v %d %s", err, recorder.Code, recorder.Body)
			}
		})
	}
}

func TestClaudeCountTokensRejectsInvalidRequestAndChannel(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"model":false}`, `{"model":"fixture","messages":[]}`} {
		c, _ := countTestContext(body, "http://unused.invalid", dto.ChannelSettings{})
		if err := ClaudeCountTokens(c); err == nil || err.StatusCode != 400 {
			t.Fatalf("invalid request accepted: %s %v", body, err)
		}
	}
	c, _ := countTestContext(`{"model":"fixture","messages":[{"role":"user","content":"hi"}]}`, "http://unused.invalid", dto.ChannelSettings{})
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	if err := ClaudeCountTokens(c); err == nil || err.StatusCode != 501 {
		t.Fatalf("unsupported channel: %v", err)
	}
}

func TestClaudeCountTokensModelAndSystemOverrides(t *testing.T) {
	raw := `{"model":"fixture","messages":[{"role":"user","content":"hi"}],"system":[{"type":"text","text":"original","cache_control":{"type":"ephemeral"},"vendor_extra":false}],"vendor_extra":{"zero":0,"large":9007199254740993,"enabled":false}}`
	c, _ := countTestContext(raw, "http://unused.invalid", dto.ChannelSettings{SystemPrompt: "prefix", SystemPromptOverride: true})
	c.Set("model_mapping", `{"fixture":"intermediate","intermediate":"mapped"}`)
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]interface{}{"metadata": map[string]interface{}{"user_id": "fixture-user"}})
	info := relaycommon.GenRelayInfoClaude(c, nil)
	info.InitChannelMeta(c)
	body, err := prepareClaudeCountTokensBody(c, info, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for path, want := range map[string]string{"model": "mapped", "system.0.text": "prefix", "system.1.text": "original", "system.1.cache_control.type": "ephemeral", "vendor_extra.large": "9007199254740993", "metadata.user_id": "fixture-user"} {
		if gjson.Get(s, path).String() != want {
			t.Errorf("%s: %s", path, s)
		}
	}
	for _, path := range []string{"vendor_extra.zero", "vendor_extra.enabled", "system.1.vendor_extra"} {
		if !gjson.Get(s, path).Exists() {
			t.Errorf("lost explicit value: %s", path)
		}
	}
	if gjson.Get(s, "max_tokens").Exists() {
		t.Fatal("generation defaults injected")
	}
	info.ChannelSetting.PassThroughBodyEnabled = true
	body, err = prepareClaudeCountTokensBody(c, info, []byte(raw))
	if err != nil || string(body) != raw {
		t.Fatalf("passthrough changed: %v %s", err, body)
	}
}

func TestClaudeCountTokensUsesChannelProxy(t *testing.T) {
	service.InitHttpClient()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "count-upstream.invalid" || r.URL.Path != "/v1/messages/count_tokens" {
			t.Errorf("wrong proxy URL: %s", r.URL)
		}
		io.WriteString(w, `{"input_tokens":9}`)
	}))
	defer proxy.Close()
	c, recorder := countTestContext(`{"model":"fixture","messages":[{"role":"user","content":"hi"}]}`, "http://count-upstream.invalid", dto.ChannelSettings{Proxy: proxy.URL})
	if err := ClaudeCountTokens(c); err != nil || recorder.Body.String() != `{"input_tokens":9}` {
		t.Fatalf("proxy failed: %v %s", err, recorder.Body)
	}
}
