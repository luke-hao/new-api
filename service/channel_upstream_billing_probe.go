package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	ChannelUpstreamBillingProbeOtherInfoKey = "upstream_billing_probe"
	channelUpstreamBillingProbeMaxKeys      = 100
	channelUpstreamBillingProbeConcurrency  = 4
	channelSub2APIBillingProbeMaxBodyBytes  = 64 * 1024
	channelNewAPIBillingProbeMaxBodyBytes   = 4 * 1024 * 1024
	channelNewAPILogPath                    = "/api/log/token"
	channelNewAPIPricingPath                = "/api/pricing"
)

var channelUpstreamBillingProbeTimeout = 8 * time.Second

var (
	channelBillingDecimalSuffixPattern = regexp.MustCompile(`^(.*?)([0-9]+\.[0-9]+)\s*$`)
	channelBillingIntegerSuffixPattern = regexp.MustCompile(`^(.*\s)([0-9]+)\s*$`)
)

type ChannelUpstreamBillingProbeStatus string

const (
	ChannelUpstreamBillingProbeStatusOK          ChannelUpstreamBillingProbeStatus = "ok"
	ChannelUpstreamBillingProbeStatusPartial     ChannelUpstreamBillingProbeStatus = "partial"
	ChannelUpstreamBillingProbeStatusUnsupported ChannelUpstreamBillingProbeStatus = "unsupported"
	ChannelUpstreamBillingProbeStatusFailed      ChannelUpstreamBillingProbeStatus = "failed"
)

type ChannelUpstreamBillingData struct {
	Object                  string   `json:"object"`
	SchemaVersion           int      `json:"schema_version"`
	BillingScope            string   `json:"billing_scope"`
	GroupRateMultiplier     float64  `json:"group_rate_multiplier"`
	UserRateMultiplier      *float64 `json:"user_rate_multiplier,omitempty"`
	ResolvedRateMultiplier  float64  `json:"resolved_rate_multiplier"`
	PeakRateEnabled         bool     `json:"peak_rate_enabled"`
	PeakStart               *string  `json:"peak_start,omitempty"`
	PeakEnd                 *string  `json:"peak_end,omitempty"`
	PeakRateMultiplier      *float64 `json:"peak_rate_multiplier,omitempty"`
	AppliedPeakMultiplier   *float64 `json:"applied_peak_multiplier,omitempty"`
	EffectiveRateMultiplier float64  `json:"effective_rate_multiplier"`
	Timezone                *string  `json:"timezone,omitempty"`
	ObservedAt              string   `json:"observed_at"`
}

type ChannelUpstreamBillingKeyResult struct {
	KeyIndex       int                               `json:"key_index"`
	KeyMask        string                            `json:"key_mask"`
	KeyFingerprint string                            `json:"key_fingerprint"`
	Status         ChannelUpstreamBillingProbeStatus `json:"status"`
	HTTPStatus     int                               `json:"http_status,omitempty"`
	ErrorCode      string                            `json:"error_code,omitempty"`
	AttemptedAt    int64                             `json:"attempted_at"`
	LastSuccessAt  int64                             `json:"last_success_at,omitempty"`
	Billing        *ChannelUpstreamBillingData       `json:"billing,omitempty"`
}

type ChannelUpstreamBillingProbeSnapshot struct {
	Status           ChannelUpstreamBillingProbeStatus `json:"status"`
	AttemptedAt      int64                             `json:"attempted_at"`
	LastSuccessAt    int64                             `json:"last_success_at,omitempty"`
	TotalKeys        int                               `json:"total_keys"`
	SuccessCount     int                               `json:"success_count"`
	UnsupportedCount int                               `json:"unsupported_count"`
	FailedCount      int                               `json:"failed_count"`
	Consistent       bool                              `json:"consistent"`
	EffectiveRateMin *float64                          `json:"effective_rate_min,omitempty"`
	EffectiveRateMax *float64                          `json:"effective_rate_max,omitempty"`
	KeyResults       []ChannelUpstreamBillingKeyResult `json:"key_results"`
	NameUpdated      bool                              `json:"name_updated,omitempty"`
	ChannelName      string                            `json:"channel_name,omitempty"`
}

type upstreamBillingProbeResponse struct {
	Object                  string   `json:"object"`
	SchemaVersion           int      `json:"schema_version"`
	BillingScope            string   `json:"billing_scope"`
	GroupRateMultiplier     *float64 `json:"group_rate_multiplier"`
	UserRateMultiplier      *float64 `json:"user_rate_multiplier"`
	ResolvedRateMultiplier  *float64 `json:"resolved_rate_multiplier"`
	PeakRateEnabled         *bool    `json:"peak_rate_enabled"`
	PeakStart               *string  `json:"peak_start"`
	PeakEnd                 *string  `json:"peak_end"`
	PeakRateMultiplier      *float64 `json:"peak_rate_multiplier"`
	AppliedPeakMultiplier   *float64 `json:"applied_peak_multiplier"`
	EffectiveRateMultiplier *float64 `json:"effective_rate_multiplier"`
	Timezone                *string  `json:"timezone"`
	ObservedAt              string   `json:"observed_at"`
}

type channelProbeKey struct {
	Index int
	Value string
}

type channelBillingProbeHTTPResponse struct {
	Body        []byte
	StatusCode  int
	ErrorCode   string
	Unsupported bool
}

type newAPILogResponse struct {
	Success *bool           `json:"success"`
	Data    []newAPILogItem `json:"data"`
}

type newAPILogItem struct {
	CreatedAt int64           `json:"created_at"`
	Group     string          `json:"group"`
	Other     json.RawMessage `json:"other"`
}

type newAPILogOther struct {
	GroupRateMultiplier *float64 `json:"group_ratio"`
	UserRateMultiplier  *float64 `json:"user_group_ratio"`
}

type newAPILogBilling struct {
	CreatedAt           int64
	Group               string
	GroupRateMultiplier float64
	UserRateMultiplier  *float64
}

type newAPIPricingResponse struct {
	Success    *bool              `json:"success"`
	GroupRatio map[string]float64 `json:"group_ratio"`
}

func BuildChannelUpstreamBillingProbeURL(baseURL string) (string, error) {
	u, err := parseChannelBillingProbeBaseURL(baseURL)
	if err != nil {
		return "", err
	}

	basePath := strings.TrimRight(u.Path, "/")
	switch {
	case strings.HasSuffix(basePath, "/v1/sub2api/billing"):
	case strings.HasSuffix(basePath, "/v1"):
		basePath += "/sub2api/billing"
	case basePath == "":
		basePath = "/v1/sub2api/billing"
	default:
		basePath += "/v1/sub2api/billing"
	}
	u.Path = basePath
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func BuildChannelNewAPIProbeURL(baseURL string, endpointPath string) (string, error) {
	if endpointPath != channelNewAPILogPath && endpointPath != channelNewAPIPricingPath {
		return "", errors.New("unsupported NewAPI probe endpoint")
	}
	u, err := parseChannelBillingProbeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	basePath := strings.TrimRight(u.Path, "/")
	switch {
	case strings.HasSuffix(basePath, endpointPath):
	case strings.HasSuffix(basePath, "/v1"):
		basePath = strings.TrimSuffix(basePath, "/v1") + endpointPath
	case basePath == "":
		basePath = endpointPath
	default:
		basePath += endpointPath
	}
	u.Path = basePath
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func parseChannelBillingProbeBaseURL(baseURL string) (*url.URL, error) {
	normalized := strings.TrimSpace(baseURL)
	if normalized == "" {
		return nil, errors.New("channel base URL is empty")
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid channel base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("channel base URL must use http or https")
	}
	if u.Host == "" || u.User != nil {
		return nil, errors.New("channel base URL must contain a host and no user info")
	}
	return u, nil
}

func DecodeChannelUpstreamBillingProbeSnapshot(otherInfo map[string]interface{}) *ChannelUpstreamBillingProbeSnapshot {
	raw, ok := otherInfo[ChannelUpstreamBillingProbeOtherInfoKey]
	if !ok || raw == nil {
		return nil
	}
	data, err := common.Marshal(raw)
	if err != nil {
		return nil
	}
	var snapshot ChannelUpstreamBillingProbeSnapshot
	if err := common.Unmarshal(data, &snapshot); err != nil {
		return nil
	}
	return &snapshot
}

func ProbeChannelUpstreamBilling(ctx context.Context, channel *model.Channel, previous *ChannelUpstreamBillingProbeSnapshot) (*ChannelUpstreamBillingProbeSnapshot, error) {
	if channel == nil {
		return nil, errors.New("channel is nil")
	}
	sub2APIProbeURL, err := BuildChannelUpstreamBillingProbeURL(channel.GetBaseURL())
	if err != nil {
		return nil, err
	}
	newAPILogURL, err := BuildChannelNewAPIProbeURL(channel.GetBaseURL(), channelNewAPILogPath)
	if err != nil {
		return nil, err
	}
	newAPIPricingURL, err := BuildChannelNewAPIProbeURL(channel.GetBaseURL(), channelNewAPIPricingPath)
	if err != nil {
		return nil, err
	}
	keys := enabledChannelProbeKeys(channel)
	if len(keys) == 0 {
		return nil, errors.New("channel has no enabled keys")
	}
	if len(keys) > channelUpstreamBillingProbeMaxKeys {
		return nil, fmt.Errorf("channel has %d enabled keys; maximum supported is %d", len(keys), channelUpstreamBillingProbeMaxKeys)
	}
	attemptedAt := time.Now().Unix()
	previousByFingerprint := make(map[string]ChannelUpstreamBillingKeyResult)
	if previous != nil {
		for _, result := range previous.KeyResults {
			previousByFingerprint[result.KeyFingerprint] = result
		}
	}

	baseClient, err := GetHttpClientWithProxy(strings.TrimSpace(channel.GetSetting().Proxy))
	if err != nil {
		results := make([]ChannelUpstreamBillingKeyResult, len(keys))
		for index, key := range keys {
			result := failedChannelProbeKeyResult(key, attemptedAt, "proxy_config_error", 0)
			preservePreviousChannelBilling(&result, previousByFingerprint)
			results[index] = result
		}
		return aggregateChannelBillingProbe(attemptedAt, results), nil
	}
	if baseClient == nil {
		baseClient = http.DefaultClient
	}
	client := *baseClient
	client.Timeout = channelUpstreamBillingProbeTimeout
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	results := make([]ChannelUpstreamBillingKeyResult, len(keys))

	semaphore := make(chan struct{}, channelUpstreamBillingProbeConcurrency)
	var wg sync.WaitGroup
	for resultIndex, key := range keys {
		resultIndex := resultIndex
		key := key
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[resultIndex] = failedChannelProbeKeyResult(key, attemptedAt, "request_cancelled", 0)
				return
			}
			result := probeChannelBillingKey(ctx, &client, sub2APIProbeURL, newAPILogURL, newAPIPricingURL, channel, key, attemptedAt)
			preservePreviousChannelBilling(&result, previousByFingerprint)
			results[resultIndex] = result
		}()
	}
	wg.Wait()

	return aggregateChannelBillingProbe(attemptedAt, results), nil
}

func preservePreviousChannelBilling(result *ChannelUpstreamBillingKeyResult, previousByFingerprint map[string]ChannelUpstreamBillingKeyResult) {
	if result == nil || result.Status == ChannelUpstreamBillingProbeStatusOK {
		return
	}
	if prior, ok := previousByFingerprint[result.KeyFingerprint]; ok && prior.Billing != nil {
		result.Billing = prior.Billing
		result.LastSuccessAt = prior.LastSuccessAt
	}
}

func enabledChannelProbeKeys(channel *model.Channel) []channelProbeKey {
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return nil
	}
	if !channel.ChannelInfo.IsMultiKey {
		return []channelProbeKey{{Index: 0, Value: strings.TrimSpace(channel.Key)}}
	}
	results := make([]channelProbeKey, 0, len(keys))
	for index, key := range keys {
		status := common.ChannelStatusEnabled
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			if configured, ok := channel.ChannelInfo.MultiKeyStatusList[index]; ok {
				status = configured
			}
		}
		if status != common.ChannelStatusEnabled {
			continue
		}
		results = append(results, channelProbeKey{Index: index, Value: strings.TrimSpace(key)})
	}
	return results
}

func probeChannelBillingKey(ctx context.Context, client *http.Client, sub2APIProbeURL string, newAPILogURL string, newAPIPricingURL string, channel *model.Channel, key channelProbeKey, attemptedAt int64) ChannelUpstreamBillingKeyResult {
	if key.Value == "" {
		return failedChannelProbeKeyResult(key, attemptedAt, "empty_key", 0)
	}
	if strings.ContainsAny(key.Value, "\r\n") {
		return failedChannelProbeKeyResult(key, attemptedAt, "unsupported_key_format", 0)
	}

	sub2APIResult := probeSub2APIChannelBillingKey(ctx, client, sub2APIProbeURL, channel, key, attemptedAt)
	if sub2APIResult.Status == ChannelUpstreamBillingProbeStatusOK {
		return sub2APIResult
	}
	newAPIResult := probeNewAPIChannelBillingKey(ctx, client, newAPILogURL, newAPIPricingURL, channel, key, attemptedAt)
	if newAPIResult.Status == ChannelUpstreamBillingProbeStatusOK {
		return newAPIResult
	}
	if sub2APIResult.Status != ChannelUpstreamBillingProbeStatusUnsupported && newAPIResult.Status == ChannelUpstreamBillingProbeStatusUnsupported {
		return sub2APIResult
	}
	return newAPIResult
}

func probeSub2APIChannelBillingKey(ctx context.Context, client *http.Client, probeURL string, channel *model.Channel, key channelProbeKey, attemptedAt int64) ChannelUpstreamBillingKeyResult {
	response := fetchChannelBillingProbeJSON(ctx, client, probeURL, channel, key.Value, channelSub2APIBillingProbeMaxBodyBytes)
	if response.ErrorCode != "" {
		result := failedChannelProbeKeyResult(key, attemptedAt, response.ErrorCode, response.StatusCode)
		if response.Unsupported {
			result.Status = ChannelUpstreamBillingProbeStatusUnsupported
		}
		return result
	}
	billing, err := parseChannelUpstreamBillingResponse(response.Body)
	if err != nil {
		return failedChannelProbeKeyResult(key, attemptedAt, "invalid_response", response.StatusCode)
	}
	return successfulChannelProbeKeyResult(key, attemptedAt, response.StatusCode, billing)
}

func probeNewAPIChannelBillingKey(ctx context.Context, client *http.Client, logURL string, pricingURL string, channel *model.Channel, key channelProbeKey, attemptedAt int64) ChannelUpstreamBillingKeyResult {
	logResponse := fetchChannelBillingProbeJSON(ctx, client, logURL, channel, key.Value, channelNewAPIBillingProbeMaxBodyBytes)
	if logResponse.ErrorCode != "" {
		result := failedChannelProbeKeyResult(key, attemptedAt, logResponse.ErrorCode, logResponse.StatusCode)
		if logResponse.Unsupported {
			result.Status = ChannelUpstreamBillingProbeStatusUnsupported
		}
		return result
	}
	logBilling, err := parseNewAPILogBillingResponse(logResponse.Body)
	if err != nil {
		return failedChannelProbeKeyResult(key, attemptedAt, "newapi_billing_log_not_found", logResponse.StatusCode)
	}

	groupRateMultiplier := logBilling.GroupRateMultiplier
	pricingResponse := fetchChannelBillingProbeJSON(ctx, client, pricingURL, channel, key.Value, channelNewAPIBillingProbeMaxBodyBytes)
	if pricingResponse.ErrorCode == "" {
		if currentGroupRate, ok := parseNewAPIPricingGroupRate(pricingResponse.Body, logBilling.Group); ok {
			groupRateMultiplier = currentGroupRate
		}
	}

	resolvedRateMultiplier := groupRateMultiplier
	userRateMultiplier := logBilling.UserRateMultiplier
	if userRateMultiplier != nil && validBillingMultiplier(*userRateMultiplier) {
		resolvedRateMultiplier = *userRateMultiplier
	} else {
		userRateMultiplier = nil
	}
	observedAt := time.Now().UTC()
	if logBilling.CreatedAt > 0 {
		observedAt = time.Unix(logBilling.CreatedAt, 0).UTC()
	}
	billing := &ChannelUpstreamBillingData{
		Object:                  "newapi.log_billing",
		SchemaVersion:           1,
		BillingScope:            "token",
		GroupRateMultiplier:     groupRateMultiplier,
		UserRateMultiplier:      userRateMultiplier,
		ResolvedRateMultiplier:  resolvedRateMultiplier,
		PeakRateEnabled:         false,
		EffectiveRateMultiplier: resolvedRateMultiplier,
		ObservedAt:              observedAt.Format(time.RFC3339Nano),
	}
	return successfulChannelProbeKeyResult(key, attemptedAt, logResponse.StatusCode, billing)
}

func successfulChannelProbeKeyResult(key channelProbeKey, attemptedAt int64, httpStatus int, billing *ChannelUpstreamBillingData) ChannelUpstreamBillingKeyResult {
	return ChannelUpstreamBillingKeyResult{
		KeyIndex:       key.Index,
		KeyMask:        model.MaskTokenKey(key.Value),
		KeyFingerprint: channelProbeKeyFingerprint(key.Value),
		Status:         ChannelUpstreamBillingProbeStatusOK,
		HTTPStatus:     httpStatus,
		AttemptedAt:    attemptedAt,
		LastSuccessAt:  attemptedAt,
		Billing:        billing,
	}
}

func fetchChannelBillingProbeJSON(ctx context.Context, client *http.Client, probeURL string, channel *model.Channel, key string, maxBodyBytes int64) channelBillingProbeHTTPResponse {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return channelBillingProbeHTTPResponse{ErrorCode: "request_build_failed"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "new-api-upstream-billing-probe/2")
	for name, rawValue := range channel.GetHeaderOverride() {
		trimmedName := strings.TrimSpace(name)
		lowerName := strings.ToLower(trimmedName)
		if trimmedName == "" || trimmedName == "*" || strings.HasPrefix(lowerName, "re:") || strings.HasPrefix(lowerName, "regex:") {
			continue
		}
		value, ok := rawValue.(string)
		if !ok || strings.ContainsAny(value, "\r\n") {
			return channelBillingProbeHTTPResponse{ErrorCode: "invalid_header_override"}
		}
		req.Header.Set(trimmedName, strings.ReplaceAll(value, "{api_key}", key))
	}

	resp, err := client.Do(req)
	if err != nil {
		var netError net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &netError) && netError.Timeout()) {
			return channelBillingProbeHTTPResponse{ErrorCode: "request_timeout"}
		}
		return channelBillingProbeHTTPResponse{ErrorCode: "request_failed"}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return channelBillingProbeHTTPResponse{StatusCode: resp.StatusCode, ErrorCode: "unsupported", Unsupported: true}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return channelBillingProbeHTTPResponse{StatusCode: resp.StatusCode, ErrorCode: "http_error"}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return channelBillingProbeHTTPResponse{StatusCode: resp.StatusCode, ErrorCode: "response_read_failed"}
	}
	if int64(len(body)) > maxBodyBytes {
		return channelBillingProbeHTTPResponse{StatusCode: resp.StatusCode, ErrorCode: "response_too_large"}
	}
	return channelBillingProbeHTTPResponse{Body: body, StatusCode: resp.StatusCode}
}

func failedChannelProbeKeyResult(key channelProbeKey, attemptedAt int64, errorCode string, httpStatus int) ChannelUpstreamBillingKeyResult {
	return ChannelUpstreamBillingKeyResult{
		KeyIndex:       key.Index,
		KeyMask:        model.MaskTokenKey(key.Value),
		KeyFingerprint: channelProbeKeyFingerprint(key.Value),
		Status:         ChannelUpstreamBillingProbeStatusFailed,
		HTTPStatus:     httpStatus,
		ErrorCode:      errorCode,
		AttemptedAt:    attemptedAt,
	}
}

func channelProbeKeyFingerprint(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:8])
}

func parseNewAPILogBillingResponse(body []byte) (*newAPILogBilling, error) {
	var response newAPILogResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Success != nil && !*response.Success {
		return nil, errors.New("NewAPI log request was not successful")
	}
	var latest *newAPILogBilling
	for _, item := range response.Data {
		group := strings.TrimSpace(item.Group)
		if group == "" {
			continue
		}
		other, err := decodeNewAPILogOther(item.Other)
		if err != nil || other.GroupRateMultiplier == nil || !validBillingMultiplier(*other.GroupRateMultiplier) {
			continue
		}
		var userRateMultiplier *float64
		if other.UserRateMultiplier != nil && validBillingMultiplier(*other.UserRateMultiplier) {
			value := *other.UserRateMultiplier
			userRateMultiplier = &value
		}
		candidate := &newAPILogBilling{
			CreatedAt:           item.CreatedAt,
			Group:               group,
			GroupRateMultiplier: *other.GroupRateMultiplier,
			UserRateMultiplier:  userRateMultiplier,
		}
		if latest == nil || candidate.CreatedAt > latest.CreatedAt {
			latest = candidate
		}
	}
	if latest == nil {
		return nil, errors.New("NewAPI log response contains no billing entry")
	}
	return latest, nil
}

func decodeNewAPILogOther(raw json.RawMessage) (*newAPILogOther, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, errors.New("NewAPI log billing details are empty")
	}
	var other newAPILogOther
	if strings.HasPrefix(trimmed, `"`) {
		var encoded string
		if err := common.Unmarshal(raw, &encoded); err != nil {
			return nil, err
		}
		if err := common.UnmarshalJsonStr(encoded, &other); err != nil {
			return nil, err
		}
		return &other, nil
	}
	if err := common.Unmarshal(raw, &other); err != nil {
		return nil, err
	}
	return &other, nil
}

func parseNewAPIPricingGroupRate(body []byte, group string) (float64, bool) {
	var response newAPIPricingResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return 0, false
	}
	if response.Success != nil && !*response.Success {
		return 0, false
	}
	rate, ok := response.GroupRatio[group]
	if !ok || !validBillingMultiplier(rate) {
		return 0, false
	}
	return rate, true
}

func ChannelUpstreamBillingNameMultiplier(snapshot *ChannelUpstreamBillingProbeSnapshot) (float64, bool) {
	if snapshot == nil || snapshot.SuccessCount == 0 || !snapshot.Consistent || snapshot.EffectiveRateMin == nil || snapshot.EffectiveRateMax == nil {
		return 0, false
	}
	if !validBillingMultiplier(*snapshot.EffectiveRateMin) || !validBillingMultiplier(*snapshot.EffectiveRateMax) || !equalBillingMultiplier(*snapshot.EffectiveRateMin, *snapshot.EffectiveRateMax) {
		return 0, false
	}
	return *snapshot.EffectiveRateMin, true
}

func FormatChannelNameWithBillingMultiplier(name string, multiplier float64) (string, bool) {
	if !validBillingMultiplier(multiplier) {
		return name, false
	}
	formattedMultiplier := strconv.FormatFloat(multiplier, 'f', -1, 64)
	if equalBillingMultiplier(multiplier, math.Trunc(multiplier)) {
		formattedMultiplier = strconv.FormatFloat(multiplier, 'f', 1, 64)
	}
	for _, pattern := range []*regexp.Regexp{channelBillingDecimalSuffixPattern, channelBillingIntegerSuffixPattern} {
		matches := pattern.FindStringSubmatch(name)
		if len(matches) != 3 {
			continue
		}
		updated := matches[1] + formattedMultiplier
		return updated, updated != name
	}
	return name, false
}

func parseChannelUpstreamBillingResponse(body []byte) (*ChannelUpstreamBillingData, error) {
	var response upstreamBillingProbeResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Object != "sub2api.key_billing" || response.SchemaVersion != 1 || response.BillingScope != "token" {
		return nil, errors.New("unexpected billing response schema")
	}
	if response.GroupRateMultiplier == nil || response.ResolvedRateMultiplier == nil || response.PeakRateEnabled == nil || response.EffectiveRateMultiplier == nil {
		return nil, errors.New("incomplete billing response")
	}
	for _, value := range []float64{*response.GroupRateMultiplier, *response.ResolvedRateMultiplier, *response.EffectiveRateMultiplier} {
		if !validBillingMultiplier(value) {
			return nil, errors.New("invalid billing multiplier")
		}
	}
	if response.UserRateMultiplier != nil && !validBillingMultiplier(*response.UserRateMultiplier) {
		return nil, errors.New("invalid user billing multiplier")
	}
	expectedResolved := *response.GroupRateMultiplier
	if response.UserRateMultiplier != nil {
		expectedResolved = *response.UserRateMultiplier
	}
	if !equalBillingMultiplier(*response.ResolvedRateMultiplier, expectedResolved) {
		return nil, errors.New("inconsistent resolved billing multiplier")
	}
	observedAt, err := time.Parse(time.RFC3339Nano, response.ObservedAt)
	if err != nil || observedAt.IsZero() {
		return nil, errors.New("invalid observed_at")
	}

	expectedPeak := 1.0
	if *response.PeakRateEnabled {
		if response.PeakStart == nil || response.PeakEnd == nil || response.PeakRateMultiplier == nil || response.AppliedPeakMultiplier == nil || response.Timezone == nil {
			return nil, errors.New("incomplete peak billing response")
		}
		if !validBillingMultiplier(*response.PeakRateMultiplier) || !validBillingMultiplier(*response.AppliedPeakMultiplier) {
			return nil, errors.New("invalid peak billing multiplier")
		}
		startMinute, ok := parseBillingClockMinute(*response.PeakStart)
		if !ok {
			return nil, errors.New("invalid peak start")
		}
		endMinute, ok := parseBillingClockMinute(*response.PeakEnd)
		if !ok || startMinute == endMinute {
			return nil, errors.New("invalid peak end")
		}
		location, err := time.LoadLocation(*response.Timezone)
		if err != nil {
			return nil, errors.New("invalid peak timezone")
		}
		localized := observedAt.In(location)
		minute := localized.Hour()*60 + localized.Minute()
		inPeakPeriod := false
		if startMinute < endMinute {
			inPeakPeriod = minute >= startMinute && minute < endMinute
		} else {
			inPeakPeriod = minute >= startMinute || minute < endMinute
		}
		if inPeakPeriod {
			expectedPeak = *response.PeakRateMultiplier
		}
		if !equalBillingMultiplier(*response.AppliedPeakMultiplier, expectedPeak) {
			return nil, errors.New("inconsistent applied peak multiplier")
		}
	} else {
		if response.PeakRateMultiplier != nil && !validBillingMultiplier(*response.PeakRateMultiplier) {
			return nil, errors.New("invalid peak billing multiplier")
		}
		if response.AppliedPeakMultiplier != nil {
			if !validBillingMultiplier(*response.AppliedPeakMultiplier) || !equalBillingMultiplier(*response.AppliedPeakMultiplier, 1) {
				return nil, errors.New("inconsistent applied peak multiplier")
			}
		}
	}
	expectedEffective := *response.ResolvedRateMultiplier * expectedPeak
	if !equalBillingMultiplier(*response.EffectiveRateMultiplier, expectedEffective) {
		return nil, errors.New("inconsistent effective billing multiplier")
	}

	return &ChannelUpstreamBillingData{
		Object:                  response.Object,
		SchemaVersion:           response.SchemaVersion,
		BillingScope:            response.BillingScope,
		GroupRateMultiplier:     *response.GroupRateMultiplier,
		UserRateMultiplier:      response.UserRateMultiplier,
		ResolvedRateMultiplier:  *response.ResolvedRateMultiplier,
		PeakRateEnabled:         *response.PeakRateEnabled,
		PeakStart:               response.PeakStart,
		PeakEnd:                 response.PeakEnd,
		PeakRateMultiplier:      response.PeakRateMultiplier,
		AppliedPeakMultiplier:   response.AppliedPeakMultiplier,
		EffectiveRateMultiplier: *response.EffectiveRateMultiplier,
		Timezone:                response.Timezone,
		ObservedAt:              observedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func parseBillingClockMinute(value string) (int, bool) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func validBillingMultiplier(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func equalBillingMultiplier(left, right float64) bool {
	scale := math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
	return math.Abs(left-right) <= 1e-9*scale
}

func aggregateChannelBillingProbe(attemptedAt int64, results []ChannelUpstreamBillingKeyResult) *ChannelUpstreamBillingProbeSnapshot {
	snapshot := &ChannelUpstreamBillingProbeSnapshot{
		AttemptedAt: attemptedAt,
		TotalKeys:   len(results),
		KeyResults:  results,
		Consistent:  true,
	}
	currentRates := make([]float64, 0, len(results))
	lastKnownRates := make([]float64, 0, len(results))
	for _, result := range results {
		switch result.Status {
		case ChannelUpstreamBillingProbeStatusOK:
			snapshot.SuccessCount++
			if result.Billing != nil {
				currentRates = append(currentRates, result.Billing.EffectiveRateMultiplier)
			}
		case ChannelUpstreamBillingProbeStatusUnsupported:
			snapshot.UnsupportedCount++
		default:
			snapshot.FailedCount++
		}
		if result.Billing != nil {
			lastKnownRates = append(lastKnownRates, result.Billing.EffectiveRateMultiplier)
		}
		if result.LastSuccessAt > snapshot.LastSuccessAt {
			snapshot.LastSuccessAt = result.LastSuccessAt
		}
	}

	switch {
	case snapshot.SuccessCount == snapshot.TotalKeys:
		snapshot.Status = ChannelUpstreamBillingProbeStatusOK
	case snapshot.SuccessCount > 0:
		snapshot.Status = ChannelUpstreamBillingProbeStatusPartial
	case snapshot.UnsupportedCount == snapshot.TotalKeys:
		snapshot.Status = ChannelUpstreamBillingProbeStatusUnsupported
	default:
		snapshot.Status = ChannelUpstreamBillingProbeStatusFailed
	}

	rates := currentRates
	if len(rates) == 0 {
		rates = lastKnownRates
	}
	if len(rates) > 0 {
		minimum := rates[0]
		maximum := rates[0]
		for _, rate := range rates[1:] {
			minimum = math.Min(minimum, rate)
			maximum = math.Max(maximum, rate)
		}
		snapshot.EffectiveRateMin = &minimum
		snapshot.EffectiveRateMax = &maximum
		snapshot.Consistent = equalBillingMultiplier(minimum, maximum)
	}
	return snapshot
}
