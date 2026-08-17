package setting

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const MaxUserGroupNameLength = 64

var userGroups = map[string]string{
	"default": "默认用户分组",
	"vip":     "VIP 用户分组",
	"svip":    "SVIP 用户分组",
}
var userGroupsMutex sync.RWMutex

func GetUserGroupsCopy() map[string]string {
	userGroupsMutex.RLock()
	defer userGroupsMutex.RUnlock()

	copyUserGroups := make(map[string]string, len(userGroups))
	for name, description := range userGroups {
		copyUserGroups[name] = description
	}
	return copyUserGroups
}

func ContainsUserGroup(name string) bool {
	userGroupsMutex.RLock()
	defer userGroupsMutex.RUnlock()
	_, ok := userGroups[name]
	return ok
}

func UserGroups2JSONString() string {
	userGroupsMutex.RLock()
	defer userGroupsMutex.RUnlock()

	jsonBytes, err := common.Marshal(userGroups)
	if err != nil {
		common.SysLog("error marshalling user identity groups: " + err.Error())
		return "{}"
	}
	return string(jsonBytes)
}

func ParseUserGroupsJSONString(jsonStr string) (map[string]string, error) {
	parsed := make(map[string]string)
	if err := common.UnmarshalJsonStr(jsonStr, &parsed); err != nil {
		return nil, err
	}
	if err := ValidateUserGroups(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ValidateUserGroups(groups map[string]string) error {
	if _, ok := groups["default"]; !ok {
		return fmt.Errorf("default user group is required")
	}
	for name := range groups {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("user group name cannot be empty")
		}
		if name != strings.TrimSpace(name) {
			return fmt.Errorf("user group name cannot start or end with whitespace: %q", name)
		}
		if utf8.RuneCountInString(name) > MaxUserGroupNameLength {
			return fmt.Errorf("user group name is longer than %d characters: %s", MaxUserGroupNameLength, name)
		}
	}
	return nil
}

func UpdateUserGroupsByJSONString(jsonStr string) error {
	parsed, err := ParseUserGroupsJSONString(jsonStr)
	if err != nil {
		return err
	}

	userGroupsMutex.Lock()
	defer userGroupsMutex.Unlock()
	userGroups = parsed
	return nil
}
