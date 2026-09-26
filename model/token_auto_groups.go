package model

import (
	"bytes"
	"fmt"
	"github.com/QuantumNous/new-api/common"
)

// TokenAutoGroups stores JSON text in SQL/Redis and exposes an ordered HTTP array.
// The zero value distinguishes omitted update fields from explicitly supplied arrays.
type TokenAutoGroups string

func (g TokenAutoGroups) Groups() []string {
	groups := []string{}
	if g != "" {
		_ = common.Unmarshal([]byte(g), &groups)
	}
	return groups
}
func (g TokenAutoGroups) MarshalJSON() ([]byte, error) { return common.Marshal(g.Groups()) }
func (g *TokenAutoGroups) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("auto_groups must be an array")
	}
	var groups []string
	if err := common.Unmarshal(data, &groups); err != nil {
		return fmt.Errorf("auto_groups must be an array of group names: %w", err)
	}
	if len(groups) > 100 {
		return fmt.Errorf("auto_groups supports at most 100 groups")
	}
	encoded, err := common.Marshal(groups)
	if err == nil {
		*g = TokenAutoGroups(encoded)
	}
	return err
}
