package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

// https://github.com/songquanpeng/one-api/issues/79

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type OpenAIUsageDailyCost struct {
	Timestamp float64 `json:"timestamp"`
	LineItems []struct {
		Name string  `json:"name"`
		Cost float64 `json:"cost"`
	}
}

type OpenAICreditGrants struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
}

type OpenAIUsageResponse struct {
	Object string `json:"object"`
	//DailyCosts []OpenAIUsageDailyCost `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 dollar
}

type OpenAISBUsageResponse struct {
	Msg  string `json:"msg"`
	Data *struct {
		Credit string `json:"credit"`
	} `json:"data"`
}

type AIProxyUserOverviewResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode int    `json:"error_code"`
	Data      struct {
		TotalPoints float64 `json:"totalPoints"`
	} `json:"data"`
}

type API2GPTUsageResponse struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalRemaining float64 `json:"total_remaining"`
}

type APGC2DGPTUsageResponse struct {
	//Grants         interface{} `json:"grants"`
	Object         string  `json:"object"`
	TotalAvailable float64 `json:"total_available"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
}

type SiliconFlowUsageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  bool   `json:"status"`
	Data    struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Image         string `json:"image"`
		Email         string `json:"email"`
		IsAdmin       bool   `json:"isAdmin"`
		Balance       string `json:"balance"`
		Status        string `json:"status"`
		Introduction  string `json:"introduction"`
		Role          string `json:"role"`
		ChargeBalance string `json:"chargeBalance"`
		TotalBalance  string `json:"totalBalance"`
		Category      string `json:"category"`
	} `json:"data"`
}

type DeepSeekUsageResponse struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

type OpenRouterCreditResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

// GetAuthHeader get auth header
func GetAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

// GetClaudeAuthHeader get claude auth header
func GetClaudeAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("x-api-key", token)
	h.Add("anthropic-version", "2023-06-01")
	return h
}

var errBalanceUnavailable = errors.New("balance endpoint unavailable")
var errBalanceOutOfRange = errors.New("balance value outside safety range")

// fetchResponse is shared by the provider-specific and compatibility probes. It
// deliberately returns the HTTP status so a provider's 404/405 can be treated as
// an unsupported balance endpoint instead of surfacing as a failed channel update.
func fetchResponse(method, rawURL string, channel *model.Channel, headers http.Header) ([]byte, int, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	for k := range headers {
		req.Header.Add(k, headers.Get(k))
	}
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	return body, res.StatusCode, nil
}

func GetResponseBody(method, rawURL string, channel *model.Channel, headers http.Header) ([]byte, error) {
	body, status, err := fetchResponse(method, rawURL, channel, headers)
	if err != nil {
		return nil, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code: %d", status)
	}
	return body, nil
}

const maxAutomaticBalanceUSD = 1_000_000

var genericBalanceKeys = map[string]struct{}{
	"balance": {}, "available_balance": {}, "current_balance": {},
	"wallet_balance": {}, "remaining_balance": {}, "total_remaining": {},
	"total_available": {}, "credits": {}, "credit": {}, "quota": {},
	"remaining": {}, "cash_balance": {}, "amount": {}, "money": {},
}

// genericBalancePaths covers the token-authenticated billing variants exposed
// by APP_A/APP_B gateways. The APP_B usage endpoint is intentionally first so
// its authoritative top-level balance wins over compatibility fallbacks.
var genericBalancePaths = []string{
	"/v1/usage",
	"/v1/sub2api/billing",
	"/api/user/balance", "/api/user/info", "/api/balance", "/api/billing/balance",
	"/api/v1/usage", "/v1/user/balance", "/v1/balance", "/balance", "/wallet/balance",
}

func parseBalanceNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(strings.Trim(v, "\""))
		s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
		s = strings.ReplaceAll(s, ",", "")
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func findGenericBalance(value any, depth int) (float64, bool) {
	if depth > 8 {
		return 0, false
	}
	if object, ok := value.(map[string]any); ok {
		// Prefer explicit balance fields at the current level before walking
		// nested data/result/payload objects.
		for key, child := range object {
			if _, ok := genericBalanceKeys[strings.ToLower(key)]; !ok {
				continue
			}
			if number, ok := parseBalanceNumber(child); ok {
				return number, true
			}
		}
		for _, child := range object {
			if number, ok := findGenericBalance(child, depth+1); ok {
				return number, true
			}
		}
	} else if list, ok := value.([]any); ok {
		for _, child := range list {
			if number, ok := findGenericBalance(child, depth+1); ok {
				return number, true
			}
		}
	}
	return 0, false
}

func validateAutomaticBalance(balance float64) error {
	if math.IsNaN(balance) || math.IsInf(balance, 0) || balance < 0 {
		return errBalanceOutOfRange
	}
	if balance > maxAutomaticBalanceUSD {
		return fmt.Errorf("%w: %.2f", errBalanceOutOfRange, balance)
	}
	return nil
}

// updateChannelGenericBalance probes the small set of balance paths commonly
// exposed by OpenAI-compatible gateways and third-party providers. Unknown
// paths, 404s, HTML pages, and provider error JSON are skipped transparently.
func updateChannelGenericBalance(channel *model.Channel) (float64, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(channel.GetBaseURL()), "/")
	if baseURL == "" {
		return 0, errBalanceUnavailable
	}
	for _, path := range genericBalancePaths {
		endpoint, err := url.JoinPath(baseURL, path)
		if err != nil {
			continue
		}
		body, status, err := fetchResponse(http.MethodGet, endpoint, channel, GetAuthHeader(channel.Key))
		if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
			continue
		}
		var payload any
		if json.Unmarshal(body, &payload) != nil {
			continue
		}
		balance, ok := findGenericBalance(payload, 0)
		if !ok || validateAutomaticBalance(balance) != nil {
			continue
		}
		channel.UpdateBalance(balance)
		return balance, nil
	}
	return 0, errBalanceUnavailable
}

func updateChannelCloseAIBalance(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("%s/dashboard/billing/credit_grants", channel.GetBaseURL())
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := OpenAICreditGrants{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelOpenAISBBalance(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("https://api.openai-sb.com/sb-api/user/status?api_key=%s", channel.Key)
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenAISBUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Data == nil {
		return 0, errors.New(response.Msg)
	}
	balance, err := strconv.ParseFloat(response.Data.Credit, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelAIProxyBalance(channel *model.Channel) (float64, error) {
	url := "https://aiproxy.io/api/report/getUserOverview"
	headers := http.Header{}
	headers.Add("Api-Key", channel.Key)
	body, err := GetResponseBody("GET", url, channel, headers)
	if err != nil {
		return 0, err
	}
	response := AIProxyUserOverviewResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if !response.Success {
		return 0, fmt.Errorf("code: %d, message: %s", response.ErrorCode, response.Message)
	}
	channel.UpdateBalance(response.Data.TotalPoints)
	return response.Data.TotalPoints, nil
}

func updateChannelAPI2GPTBalance(channel *model.Channel) (float64, error) {
	url := "https://api.api2gpt.com/dashboard/billing/credit_grants"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := API2GPTUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalRemaining)
	return response.TotalRemaining, nil
}

func updateChannelSiliconFlowBalance(channel *model.Channel) (float64, error) {
	url := "https://api.siliconflow.cn/v1/user/info"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := SiliconFlowUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Code != 20000 {
		return 0, fmt.Errorf("code: %d, message: %s", response.Code, response.Message)
	}
	balance, err := strconv.ParseFloat(response.Data.TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelDeepSeekBalance(channel *model.Channel) (float64, error) {
	url := "https://api.deepseek.com/user/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := DeepSeekUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	index := -1
	for i, balanceInfo := range response.BalanceInfos {
		if balanceInfo.Currency == "CNY" {
			index = i
			break
		}
	}
	if index == -1 {
		return 0, errors.New("currency CNY not found")
	}
	balance, err := strconv.ParseFloat(response.BalanceInfos[index].TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelAIGC2DBalance(channel *model.Channel) (float64, error) {
	url := "https://api.aigc2d.com/dashboard/billing/credit_grants"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := APGC2DGPTUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelOpenRouterBalance(channel *model.Channel) (float64, error) {
	url := "https://openrouter.ai/api/v1/credits"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenRouterCreditResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	balance := response.Data.TotalCredits - response.Data.TotalUsage
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelMoonshotBalance(channel *model.Channel) (float64, error) {
	url := "https://api.moonshot.cn/v1/users/me/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}

	type MoonshotBalanceData struct {
		AvailableBalance float64 `json:"available_balance"`
		VoucherBalance   float64 `json:"voucher_balance"`
		CashBalance      float64 `json:"cash_balance"`
	}

	type MoonshotBalanceResponse struct {
		Code   int                 `json:"code"`
		Data   MoonshotBalanceData `json:"data"`
		Scode  string              `json:"scode"`
		Status bool                `json:"status"`
	}

	response := MoonshotBalanceResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if !response.Status || response.Code != 0 {
		return 0, fmt.Errorf("failed to update moonshot balance, status: %v, code: %d, scode: %s", response.Status, response.Code, response.Scode)
	}
	availableBalanceCny := response.Data.AvailableBalance
	availableBalanceUsd := decimal.NewFromFloat(availableBalanceCny).Div(decimal.NewFromFloat(operation_setting.Price)).InexactFloat64()
	channel.UpdateBalance(availableBalanceUsd)
	return availableBalanceUsd, nil
}

func updateChannelOpenAICompatibleBalance(channel *model.Channel) (float64, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(channel.GetBaseURL()), "/")
	if baseURL == "" {
		return 0, errBalanceUnavailable
	}
	subscriptionURL := baseURL + "/v1/dashboard/billing/subscription"
	body, err := GetResponseBody(http.MethodGet, subscriptionURL, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	subscription := OpenAISubscriptionResponse{}
	if err = json.Unmarshal(body, &subscription); err != nil {
		return 0, err
	}
	// A number of gateways return a synthetic 100-million-dollar hard limit.
	// Treat it as an unimplemented billing endpoint rather than writing a
	// fabricated balance into the channel.
	if err = validateAutomaticBalance(subscription.HardLimitUSD); err != nil {
		return 0, err
	}
	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	usageURL := fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate)
	body, err = GetResponseBody(http.MethodGet, usageURL, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	usage := OpenAIUsageResponse{}
	if err = json.Unmarshal(body, &usage); err != nil {
		return 0, err
	}
	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	if err = validateAutomaticBalance(balance); err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelBalance(channel *model.Channel) (float64, error) {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() == "" {
		channel.BaseURL = &baseURL
	}
	var primary func(*model.Channel) (float64, error)
	switch channel.Type {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeCustom:
		primary = updateChannelOpenAICompatibleBalance
	case constant.ChannelTypeAIProxy:
		primary = updateChannelAIProxyBalance
	case constant.ChannelTypeAPI2GPT:
		primary = updateChannelAPI2GPTBalance
	case constant.ChannelTypeAIGC2D:
		primary = updateChannelAIGC2DBalance
	case constant.ChannelTypeSiliconFlow:
		primary = updateChannelSiliconFlowBalance
	case constant.ChannelTypeDeepSeek:
		primary = updateChannelDeepSeekBalance
	case constant.ChannelTypeOpenRouter:
		primary = updateChannelOpenRouterBalance
	case constant.ChannelTypeMoonshot:
		primary = updateChannelMoonshotBalance
	}
	if primary != nil {
		if balance, err := primary(channel); err == nil {
			return balance, nil
		}
	}
	// Provider-specific endpoints are not uniform. Probe common alternatives
	// for every channel type before reporting the balance as unavailable.
	return updateChannelGenericBalance(channel)
}

func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.CacheGetChannel(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "多密钥渠道不支持余额查询",
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		if errors.Is(err, errBalanceUnavailable) || errors.Is(err, errBalanceOutOfRange) {
			// Keep the last confirmed value and make the operation idempotent for
			// providers that do not publish a billing endpoint (often 404/405).
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"updated": false,
				"message": "\u4f59\u989d\u63a5\u53e3\u4e0d\u53ef\u7528\uff0c\u5df2\u4fdd\u7559\u4e0a\u6b21\u4f59\u989d",
				"balance": channel.Balance,
			})
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"updated": true,
		"message": "",
		"balance": balance,
	})
}

func updateAllChannelsBalance() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if channel.ChannelInfo.IsMultiKey {
			continue // skip multi-key channels
		}
		// TODO: support Azure
		//if channel.Type != common.ChannelTypeOpenAI && channel.Type != common.ChannelTypeCustom {
		//	continue
		//}
		balance, err := updateChannelBalance(channel)
		if err != nil {
			continue
		} else {
			// err is nil & balance <= 0 means quota is used up
			if balance <= 0 {
				service.DisableChannel(*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()), "余额不足")
			}
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func UpdateAllChannelsBalance(c *gin.Context) {
	// TODO: make it async
	err := updateAllChannelsBalance()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func AutomaticallyUpdateChannels(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		common.SysLog("updating all channels")
		_ = updateAllChannelsBalance()
		common.SysLog("channels update done")
	}
}
