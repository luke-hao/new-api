package ratio_setting

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	imageTokenBillingGroupsMu sync.RWMutex
	imageTokenBillingGroups   = map[string]struct{}{}
)

func ParseImageTokenBillingGroups(jsonStr string) ([]string, error) {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "[]"
	}
	var groups []string
	if err := json.Unmarshal([]byte(jsonStr), &groups); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group) == "" || group != strings.TrimSpace(group) {
			return nil, fmt.Errorf("invalid image token billing group: %q", group)
		}
		if _, ok := seen[group]; ok {
			return nil, fmt.Errorf("duplicate image token billing group: %s", group)
		}
		seen[group] = struct{}{}
	}
	return groups, nil
}

func ImageTokenBillingGroups2JSONString() string {
	imageTokenBillingGroupsMu.RLock()
	defer imageTokenBillingGroupsMu.RUnlock()
	groups := make([]string, 0, len(imageTokenBillingGroups))
	for group := range imageTokenBillingGroups {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	value, _ := json.Marshal(groups)
	return string(value)
}

func UpdateImageTokenBillingGroupsByJSONString(jsonStr string) error {
	groups, err := ParseImageTokenBillingGroups(jsonStr)
	if err != nil {
		return err
	}
	next := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		next[group] = struct{}{}
	}
	imageTokenBillingGroupsMu.Lock()
	imageTokenBillingGroups = next
	imageTokenBillingGroupsMu.Unlock()
	return nil
}

func IsImageTokenBillingGroup(group string) bool {
	imageTokenBillingGroupsMu.RLock()
	defer imageTokenBillingGroupsMu.RUnlock()
	_, ok := imageTokenBillingGroups[group]
	return ok
}
