package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestBuildChannelUpstreamBillingProbeURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com":                              "https://example.com/v1/sub2api/billing",
		"https://example.com/":                             "https://example.com/v1/sub2api/billing",
		"https://example.com/v1":                           "https://example.com/v1/sub2api/billing",
		"https://example.com/openai":                       "https://example.com/openai/v1/sub2api/billing",
		"https://example.com/openai/v1/":                   "https://example.com/openai/v1/sub2api/billing",
		"https://example.com/v1/sub2api/billing?ignored=1": "https://example.com/v1/sub2api/billing",
	}
	for input, expected := range tests {
		actual, err := BuildChannelUpstreamBillingProbeURL(input)
		if err != nil {
			t.Fatalf("BuildChannelUpstreamBillingProbeURL(%q): %v", input, err)
		}
		if actual != expected {
			t.Fatalf("BuildChannelUpstreamBillingProbeURL(%q) = %q, want %q", input, actual, expected)
		}
	}
	for _, input := range []string{"", "ftp://example.com", "https:///missing-host", "https://user@example.com"} {
		if _, err := BuildChannelUpstreamBillingProbeURL(input); err == nil {
			t.Fatalf("expected invalid URL %q to fail", input)
		}
	}
}

func TestBuildChannelNewAPIProbeURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com":                     "https://example.com/api/log/token",
		"https://example.com/":                    "https://example.com/api/log/token",
		"https://example.com/v1":                  "https://example.com/api/log/token",
		"https://example.com/openai":              "https://example.com/openai/api/log/token",
		"https://example.com/openai/v1/":          "https://example.com/openai/api/log/token",
		"https://example.com/api/log/token?q=one": "https://example.com/api/log/token",
	}
	for input, expected := range tests {
		actual, err := BuildChannelNewAPIProbeURL(input, channelNewAPILogPath)
		if err != nil {
			t.Fatalf("BuildChannelNewAPIProbeURL(%q): %v", input, err)
		}
		if actual != expected {
			t.Fatalf("BuildChannelNewAPIProbeURL(%q) = %q, want %q", input, actual, expected)
		}
	}
	pricingURL, err := BuildChannelNewAPIProbeURL("https://example.com/v1", channelNewAPIPricingPath)
	if err != nil || pricingURL != "https://example.com/api/pricing" {
		t.Fatalf("unexpected pricing URL: %q, %v", pricingURL, err)
	}
	if _, err := BuildChannelNewAPIProbeURL("https://example.com", "/api/unknown"); err == nil {
		t.Fatal("expected unsupported endpoint to fail")
	}
}

func TestParseChannelUpstreamBillingResponse(t *testing.T) {
	observed := "2026-07-28T12:00:00Z"
	tests := []struct {
		name    string
		payload string
		wantErr bool
		want    float64
	}{
		{
			name:    "plain group rate",
			payload: billingPayload(observed, `"group_rate_multiplier":0.5,"resolved_rate_multiplier":0.5,"peak_rate_enabled":false,"effective_rate_multiplier":0.5`),
			want:    0.5,
		},
		{
			name:    "user and peak rate",
			payload: billingPayload(observed, `"group_rate_multiplier":1,"user_rate_multiplier":0.4,"resolved_rate_multiplier":0.4,"peak_rate_enabled":true,"peak_start":"11:00","peak_end":"13:00","peak_rate_multiplier":2,"applied_peak_multiplier":2,"effective_rate_multiplier":0.8,"timezone":"UTC"`),
			want:    0.8,
		},
		{
			name:    "overnight peak rate",
			payload: billingPayload(observed, `"group_rate_multiplier":0.5,"resolved_rate_multiplier":0.5,"peak_rate_enabled":true,"peak_start":"23:00","peak_end":"01:00","peak_rate_multiplier":2,"applied_peak_multiplier":1,"effective_rate_multiplier":0.5,"timezone":"UTC"`),
			want:    0.5,
		},
		{
			name:    "negative rate",
			payload: billingPayload(observed, `"group_rate_multiplier":-1,"resolved_rate_multiplier":-1,"peak_rate_enabled":false,"effective_rate_multiplier":-1`),
			wantErr: true,
		},
		{
			name:    "inconsistent resolved rate",
			payload: billingPayload(observed, `"group_rate_multiplier":1,"user_rate_multiplier":0.4,"resolved_rate_multiplier":0.5,"peak_rate_enabled":false,"effective_rate_multiplier":0.5`),
			wantErr: true,
		},
		{
			name:    "inconsistent effective rate",
			payload: billingPayload(observed, `"group_rate_multiplier":1,"resolved_rate_multiplier":1,"peak_rate_enabled":true,"peak_start":"11:00","peak_end":"13:00","peak_rate_multiplier":2,"applied_peak_multiplier":2,"effective_rate_multiplier":1,"timezone":"UTC"`),
			wantErr: true,
		},
		{
			name:    "peak disabled but applied multiplier is not one",
			payload: billingPayload(observed, `"group_rate_multiplier":1,"resolved_rate_multiplier":1,"peak_rate_enabled":false,"applied_peak_multiplier":2,"effective_rate_multiplier":1`),
			wantErr: true,
		},
		{
			name:    "unexpected schema",
			payload: `{"object":"other","schema_version":1,"billing_scope":"token","group_rate_multiplier":1,"resolved_rate_multiplier":1,"peak_rate_enabled":false,"effective_rate_multiplier":1,"observed_at":"2026-07-28T12:00:00Z"}`,
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := parseChannelUpstreamBillingResponse([]byte(test.payload))
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if data.EffectiveRateMultiplier != test.want {
				t.Fatalf("effective rate = %v, want %v", data.EffectiveRateMultiplier, test.want)
			}
		})
	}
}

func TestProbeChannelUpstreamBillingMultiKeyPartialPreservesLastSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sub2api/billing" {
			http.NotFound(w, r)
			return
		}
		switch r.Header.Get("Authorization") {
		case "Bearer key-a":
			_, _ = w.Write([]byte(billingPayload("2026-07-28T12:00:00Z", `"group_rate_multiplier":0.5,"resolved_rate_multiplier":0.5,"peak_rate_enabled":false,"effective_rate_multiplier":0.5`)))
		case "Bearer key-b":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Id:      7,
		Key:     "key-a\nkey-b\nkey-disabled",
		BaseURL: common.GetPointer(server.URL),
		ChannelInfo: model.ChannelInfo{
			IsMultiKey: true,
			MultiKeyStatusList: map[int]int{
				2: common.ChannelStatusManuallyDisabled,
			},
		},
	}
	previous := &ChannelUpstreamBillingProbeSnapshot{
		KeyResults: []ChannelUpstreamBillingKeyResult{
			{
				KeyIndex:       1,
				KeyFingerprint: channelProbeKeyFingerprint("key-b"),
				Status:         ChannelUpstreamBillingProbeStatusOK,
				LastSuccessAt:  1234,
				Billing: &ChannelUpstreamBillingData{
					EffectiveRateMultiplier: 0.8,
				},
			},
		},
	}

	snapshot, err := ProbeChannelUpstreamBilling(context.Background(), channel, previous)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != ChannelUpstreamBillingProbeStatusPartial || snapshot.TotalKeys != 2 || snapshot.SuccessCount != 1 || snapshot.FailedCount != 1 {
		t.Fatalf("unexpected snapshot summary: %+v", snapshot)
	}
	if snapshot.EffectiveRateMin == nil || *snapshot.EffectiveRateMin != 0.5 || snapshot.EffectiveRateMax == nil || *snapshot.EffectiveRateMax != 0.5 {
		t.Fatalf("unexpected current rate range: %+v", snapshot)
	}
	failed := snapshot.KeyResults[1]
	if failed.Billing == nil || failed.Billing.EffectiveRateMultiplier != 0.8 || failed.LastSuccessAt != 1234 {
		t.Fatalf("previous success was not preserved: %+v", failed)
	}
}

func TestProbeChannelUpstreamBillingFallsBackToNewAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer newapi-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/sub2api/billing":
			http.NotFound(w, r)
		case "/api/log/token":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"created_at":200,"group":"vip","other":{"channel_id":1,"error_code":"upstream_error"}},{"created_at":180,"group":"vip","other":"{\"group_ratio\":0.25,\"user_group_ratio\":-1}"},{"created_at":100,"group":"default","other":{"group_ratio":1,"user_group_ratio":-1}}]}`))
		case "/api/pricing":
			_, _ = w.Write([]byte(`{"success":true,"group_ratio":{"vip":0.35,"default":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	channel := &model.Channel{Id: 1, Key: "newapi-key", BaseURL: common.GetPointer(server.URL + "/v1")}
	snapshot, err := ProbeChannelUpstreamBilling(context.Background(), channel, nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != ChannelUpstreamBillingProbeStatusOK || snapshot.SuccessCount != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	billing := snapshot.KeyResults[0].Billing
	if billing == nil || billing.Object != "newapi.log_billing" || billing.GroupRateMultiplier != 0.35 || billing.ResolvedRateMultiplier != 0.35 || billing.EffectiveRateMultiplier != 0.35 {
		t.Fatalf("unexpected NewAPI billing: %+v", billing)
	}
	if billing.ObservedAt != "1970-01-01T00:03:00Z" {
		t.Fatalf("unexpected observed time: %q", billing.ObservedAt)
	}
}

func TestProbeChannelUpstreamBillingNewAPIUserGroupRateWins(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sub2api/billing":
			http.NotFound(w, r)
		case "/api/log/token":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"created_at":200,"group":"vip","other":{"group_ratio":0.25,"user_group_ratio":0.5}}]}`))
		case "/api/pricing":
			_, _ = w.Write([]byte(`{"success":true,"group_ratio":{"vip":0.8}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	channel := &model.Channel{Id: 1, Key: "newapi-key", BaseURL: common.GetPointer(server.URL)}
	snapshot, err := ProbeChannelUpstreamBilling(context.Background(), channel, nil)
	if err != nil {
		t.Fatal(err)
	}
	billing := snapshot.KeyResults[0].Billing
	if billing == nil || billing.GroupRateMultiplier != 0.8 || billing.UserRateMultiplier == nil || *billing.UserRateMultiplier != 0.5 || billing.EffectiveRateMultiplier != 0.5 {
		t.Fatalf("unexpected special user rate: %+v", billing)
	}
}

func TestFormatChannelNameWithBillingMultiplier(t *testing.T) {
	tests := []struct {
		name       string
		multiplier float64
		want       string
		changed    bool
	}{
		{name: "lin 0.25", multiplier: 0.35, want: "lin 0.35", changed: true},
		{name: "lin_limit0.25", multiplier: 0.35, want: "lin_limit0.35", changed: true},
		{name: "foo 0.25", multiplier: 1, want: "foo 1.0", changed: true},
		{name: "foo 2", multiplier: 1, want: "foo 1.0", changed: true},
		{name: "channel7", multiplier: 0.5, want: "channel7", changed: false},
		{name: "cpamc stable", multiplier: 0.5, want: "cpamc stable", changed: false},
		{name: "foo 1.0", multiplier: 1, want: "foo 1.0", changed: false},
	}
	for _, test := range tests {
		actual, changed := FormatChannelNameWithBillingMultiplier(test.name, test.multiplier)
		if actual != test.want || changed != test.changed {
			t.Fatalf("FormatChannelNameWithBillingMultiplier(%q, %v) = %q, %v; want %q, %v", test.name, test.multiplier, actual, changed, test.want, test.changed)
		}
	}
}

func TestChannelUpstreamBillingNameMultiplier(t *testing.T) {
	rate := 0.35
	snapshot := &ChannelUpstreamBillingProbeSnapshot{
		SuccessCount:     1,
		Consistent:       true,
		EffectiveRateMin: &rate,
		EffectiveRateMax: &rate,
	}
	if actual, ok := ChannelUpstreamBillingNameMultiplier(snapshot); !ok || actual != rate {
		t.Fatalf("unexpected name multiplier: %v, %v", actual, ok)
	}
	snapshot.SuccessCount = 0
	if _, ok := ChannelUpstreamBillingNameMultiplier(snapshot); ok {
		t.Fatal("last-known values without current success must not rename a channel")
	}
}

func TestProbeChannelUpstreamBillingHTTPOutcomes(t *testing.T) {
	originalTimeout := channelUpstreamBillingProbeTimeout
	channelUpstreamBillingProbeTimeout = 40 * time.Millisecond
	defer func() { channelUpstreamBillingProbeTimeout = originalTimeout }()

	var redirectTargetHits atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectTargetHits.Add(1)
		_, _ = w.Write([]byte(billingPayload("2026-07-28T12:00:00Z", `"group_rate_multiplier":1,"resolved_rate_multiplier":1,"peak_rate_enabled":false,"effective_rate_multiplier":1`)))
	}))
	defer redirectTarget.Close()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus ChannelUpstreamBillingProbeStatus
		wantError  string
	}{
		{"unsupported", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "", http.StatusNotFound) }, ChannelUpstreamBillingProbeStatusUnsupported, "unsupported"},
		{"unsupported method", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "", http.StatusMethodNotAllowed) }, ChannelUpstreamBillingProbeStatusUnsupported, "unsupported"},
		{"http error", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "", http.StatusTooManyRequests) }, ChannelUpstreamBillingProbeStatusFailed, "http_error"},
		{"too large", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", channelNewAPIBillingProbeMaxBodyBytes+1)))
		}, ChannelUpstreamBillingProbeStatusFailed, "response_too_large"},
		{"timeout", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}, ChannelUpstreamBillingProbeStatusFailed, "request_timeout"},
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
		}, ChannelUpstreamBillingProbeStatusFailed, "http_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			channel := &model.Channel{Id: 1, Key: "secret-key", BaseURL: common.GetPointer(server.URL)}
			snapshot, err := ProbeChannelUpstreamBilling(context.Background(), channel, nil)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Status != test.wantStatus || len(snapshot.KeyResults) != 1 || snapshot.KeyResults[0].ErrorCode != test.wantError {
				t.Fatalf("unexpected result: %+v", snapshot)
			}
		})
	}
	if redirectTargetHits.Load() != 0 {
		t.Fatalf("redirect target received %d requests", redirectTargetHits.Load())
	}
}

func TestChannelProbeKeyIdentityIsSafeAndStable(t *testing.T) {
	key := "sk-sensitive-secret-value"
	masked := model.MaskTokenKey(key)
	fingerprint := channelProbeKeyFingerprint(key)
	if strings.Contains(masked, "sensitive-secret") || masked == key {
		t.Fatalf("key mask exposes the key: %q", masked)
	}
	if len(fingerprint) != 16 || fingerprint != channelProbeKeyFingerprint(key) {
		t.Fatalf("unexpected fingerprint: %q", fingerprint)
	}
	if fingerprint == channelProbeKeyFingerprint(key+"-different") {
		t.Fatal("different keys produced the same test fingerprint")
	}
}

func TestProbeChannelUpstreamBillingRejectsMoreThanOneHundredEnabledKeys(t *testing.T) {
	keys := make([]string, channelUpstreamBillingProbeMaxKeys+1)
	for index := range keys {
		keys[index] = fmt.Sprintf("key-%d", index)
	}
	channel := &model.Channel{
		Key:     strings.Join(keys, "\n"),
		BaseURL: common.GetPointer("https://example.com"),
		ChannelInfo: model.ChannelInfo{
			IsMultiKey: true,
		},
	}
	if _, err := ProbeChannelUpstreamBilling(context.Background(), channel, nil); err == nil {
		t.Fatal("expected too many enabled keys to fail validation")
	}
}

func billingPayload(observedAt, fields string) string {
	return fmt.Sprintf(`{"object":"sub2api.key_billing","schema_version":1,"billing_scope":"token",%s,"observed_at":%q}`, fields, observedAt)
}
