package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	invalidClaudeThinkingKeyPrefix = "new-api:claude:invalid-thinking:"
	invalidClaudeThinkingTTL       = 30 * 24 * time.Hour
	maxRememberedThinkingBlocks    = 512

	contextKeyThinkingPreflightRemoved = "claude_thinking_preflight_removed"
	contextKeyThinkingRecoveryAttempt  = "claude_thinking_recovery_attempt"
	contextKeyThinkingRecoveryRemoved  = "claude_thinking_recovery_removed"
	contextKeyThinkingRecoverySuccess  = "claude_thinking_recovery_success"
)

type invalidClaudeThinkingCacheEntry struct {
	expiresAt int64
}

var invalidClaudeThinkingCache sync.Map

// ClaudeThinkingSanitizeResult describes an in-memory request rewrite. It never
// contains the thinking text or its opaque signature.
type ClaudeThinkingSanitizeResult struct {
	Body            []byte
	Fingerprints    []string
	RemovedBlocks   int
	RemovedMessages int
}

// IsInvalidClaudeThinkingSignatureError matches only the upstream validation
// failure that can be recovered by removing historical signed thinking blocks.
func IsInvalidClaudeThinkingSignatureError(err *types.NewAPIError) bool {
	if err == nil || err.StatusCode != http.StatusBadRequest {
		return false
	}
	message := strings.ToLower(common.StripRequestIDAnnotations(err.Error()))
	return strings.Contains(message, "invalid signature in thinking block")
}

// SanitizeKnownInvalidClaudeThinking removes blocks that were learned from a
// previous upstream signature failure. Redis makes the knowledge survive a
// restart, while the in-process cache keeps the normal path inexpensive.
func SanitizeKnownInvalidClaudeThinking(body []byte) (ClaudeThinkingSanitizeResult, error) {
	scan, err := rewriteClaudeThinkingBlocks(body, nil, false)
	if err != nil || len(scan.Fingerprints) == 0 {
		return scan, err
	}
	invalid := lookupInvalidClaudeThinking(scan.Fingerprints)
	if len(invalid) == 0 {
		return scan, nil
	}
	return rewriteClaudeThinkingBlocks(body, invalid, false)
}

// SanitizeAllClaudeThinking removes every historical thinking and
// redacted_thinking content block after the upstream has proven that at least
// one signature in the request is incompatible with the selected channel.
func SanitizeAllClaudeThinking(body []byte) (ClaudeThinkingSanitizeResult, error) {
	return rewriteClaudeThinkingBlocks(body, nil, true)
}

func rewriteClaudeThinkingBlocks(body []byte, invalid map[string]struct{}, removeAll bool) (ClaudeThinkingSanitizeResult, error) {
	result := ClaudeThinkingSanitizeResult{Body: body}
	var root map[string]json.RawMessage
	if err := common.Unmarshal(body, &root); err != nil {
		return result, err
	}
	messagesJSON, ok := root["messages"]
	if !ok {
		return result, nil
	}

	var messages []map[string]json.RawMessage
	if err := common.Unmarshal(messagesJSON, &messages); err != nil {
		return result, err
	}

	rewrittenMessages := make([]map[string]json.RawMessage, 0, len(messages))
	fingerprints := make(map[string]struct{})
	for _, message := range messages {
		contentJSON, hasContent := message["content"]
		if !hasContent || !rawJSONArray(contentJSON) {
			rewrittenMessages = append(rewrittenMessages, message)
			continue
		}

		var blocks []json.RawMessage
		if err := common.Unmarshal(contentJSON, &blocks); err != nil {
			return result, err
		}
		kept := make([]json.RawMessage, 0, len(blocks))
		messageChanged := false
		for _, block := range blocks {
			blockType, fingerprint, isThinking := claudeThinkingBlockFingerprint(block)
			if !isThinking {
				kept = append(kept, block)
				continue
			}
			_ = blockType
			if fingerprint != "" {
				fingerprints[fingerprint] = struct{}{}
			}
			_, knownInvalid := invalid[fingerprint]
			if removeAll || knownInvalid {
				result.RemovedBlocks++
				messageChanged = true
				continue
			}
			kept = append(kept, block)
		}

		if !messageChanged {
			rewrittenMessages = append(rewrittenMessages, message)
			continue
		}
		if len(kept) == 0 {
			result.RemovedMessages++
			continue
		}
		content, err := common.Marshal(kept)
		if err != nil {
			return result, err
		}
		message["content"] = content
		rewrittenMessages = append(rewrittenMessages, message)
	}

	result.Fingerprints = sortedBoundedFingerprints(fingerprints)
	if result.RemovedBlocks == 0 {
		return result, nil
	}
	messagesBody, err := common.Marshal(rewrittenMessages)
	if err != nil {
		return result, err
	}
	root["messages"] = messagesBody
	result.Body, err = common.Marshal(root)
	return result, err
}

func rawJSONArray(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "[")
}

func claudeThinkingBlockFingerprint(block json.RawMessage) (blockType string, fingerprint string, isThinking bool) {
	var meta struct {
		Type      string          `json:"type"`
		Signature string          `json:"signature"`
		Data      json.RawMessage `json:"data"`
	}
	if err := common.Unmarshal(block, &meta); err != nil {
		return "", "", false
	}
	if meta.Type != "thinking" && meta.Type != "redacted_thinking" {
		return meta.Type, "", false
	}

	payload := []byte(meta.Type + "\x00")
	if meta.Type == "thinking" && meta.Signature != "" {
		payload = append(payload, meta.Signature...)
	} else if meta.Type == "redacted_thinking" && len(meta.Data) > 0 {
		payload = append(payload, meta.Data...)
	} else {
		payload = append(payload, block...)
	}
	digest := sha256.Sum256(payload)
	return meta.Type, stringHex(digest[:]), true
}

func stringHex(data []byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, len(data)*2)
	for i, value := range data {
		encoded[i*2] = digits[value>>4]
		encoded[i*2+1] = digits[value&0x0f]
	}
	return string(encoded)
}

func sortedBoundedFingerprints(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	// Stable order keeps tests and diagnostics deterministic without exposing the
	// underlying opaque signatures.
	sort.Strings(result)
	if len(result) > maxRememberedThinkingBlocks {
		result = result[:maxRememberedThinkingBlocks]
	}
	return result
}

// RememberInvalidClaudeThinking stores only SHA-256 fingerprints. Cache errors
// are deliberately best-effort because recovery has already removed the bad
// blocks from the current request.
func RememberInvalidClaudeThinking(fingerprints []string) {
	fingerprints = boundedUniqueStrings(fingerprints)
	if len(fingerprints) == 0 {
		return
	}
	expiresAt := time.Now().Add(invalidClaudeThinkingTTL).UnixNano()
	for _, fingerprint := range fingerprints {
		invalidClaudeThinkingCache.Store(fingerprint, invalidClaudeThinkingCacheEntry{expiresAt: expiresAt})
	}
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	pipeline := common.RDB.Pipeline()
	for _, fingerprint := range fingerprints {
		pipeline.Set(ctx, invalidClaudeThinkingKeyPrefix+fingerprint, "1", invalidClaudeThinkingTTL)
	}
	_, _ = pipeline.Exec(ctx)
}

func lookupInvalidClaudeThinking(fingerprints []string) map[string]struct{} {
	fingerprints = boundedUniqueStrings(fingerprints)
	invalid := make(map[string]struct{})
	if len(fingerprints) == 0 {
		return invalid
	}

	now := time.Now().UnixNano()
	unknown := make([]string, 0, len(fingerprints))
	for _, fingerprint := range fingerprints {
		if cached, ok := invalidClaudeThinkingCache.Load(fingerprint); ok {
			entry, valid := cached.(invalidClaudeThinkingCacheEntry)
			if valid && entry.expiresAt > now {
				invalid[fingerprint] = struct{}{}
				continue
			}
			invalidClaudeThinkingCache.Delete(fingerprint)
		}
		unknown = append(unknown, fingerprint)
	}
	if len(unknown) == 0 || !common.RedisEnabled || common.RDB == nil {
		return invalid
	}

	keys := make([]string, 0, len(unknown))
	for _, fingerprint := range unknown {
		keys = append(keys, invalidClaudeThinkingKeyPrefix+fingerprint)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	values, err := common.RDB.MGet(ctx, keys...).Result()
	if err != nil {
		return invalid
	}
	expiresAt := time.Now().Add(invalidClaudeThinkingTTL).UnixNano()
	for i, value := range values {
		if value == nil {
			continue
		}
		fingerprint := unknown[i]
		invalid[fingerprint] = struct{}{}
		invalidClaudeThinkingCache.Store(fingerprint, invalidClaudeThinkingCacheEntry{expiresAt: expiresAt})
	}
	return invalid
}

func boundedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
		if len(result) == maxRememberedThinkingBlocks {
			break
		}
	}
	return result
}

func MarkClaudeThinkingPreflightRemoved(c *gin.Context, removed int) {
	if c == nil || removed <= 0 {
		return
	}
	c.Set(contextKeyThinkingPreflightRemoved, c.GetInt(contextKeyThinkingPreflightRemoved)+removed)
}

func MarkClaudeThinkingRecoveryAttempt(c *gin.Context, removed int) {
	if c == nil {
		return
	}
	c.Set(contextKeyThinkingRecoveryAttempt, true)
	c.Set(contextKeyThinkingRecoveryRemoved, removed)
}

func MarkClaudeThinkingRecoverySuccess(c *gin.Context) {
	if c != nil {
		c.Set(contextKeyThinkingRecoverySuccess, true)
	}
}

func AppendClaudeThinkingRecoveryAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	preflightRemoved := c.GetInt(contextKeyThinkingPreflightRemoved)
	attempted := c.GetBool(contextKeyThinkingRecoveryAttempt)
	if preflightRemoved == 0 && !attempted {
		return
	}
	recovery := map[string]interface{}{}
	if preflightRemoved > 0 {
		recovery["preflight_removed_blocks"] = preflightRemoved
	}
	if attempted {
		recovery["attempted"] = true
		recovery["removed_blocks"] = c.GetInt(contextKeyThinkingRecoveryRemoved)
		recovery["success"] = c.GetBool(contextKeyThinkingRecoverySuccess)
	}
	adminInfo["thinking_signature_recovery"] = recovery
}
