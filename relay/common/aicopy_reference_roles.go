package common

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
)

// validateAICopyImageRoles enforces the supplied public API's role contract
// before quota reservation or any upload/create request.
func validateAICopyImageRoles(metadata map[string]any) error {
	value, exists := metadata["reference_images"]
	if !exists {
		return nil
	}
	raw, err := common.Marshal(value)
	if err != nil {
		return fmt.Errorf("reference_images is invalid")
	}
	var refs []any
	if common.Unmarshal(raw, &refs) != nil {
		return fmt.Errorf("reference_images is invalid")
	}
	first, last, regular := 0, 0, 0
	for _, ref := range refs {
		role := ""
		if object, ok := ref.(map[string]any); ok {
			if value, exists := object["role"]; exists {
				var valid bool
				role, valid = value.(string)
				if !valid {
					return fmt.Errorf("image role must be a string")
				}
			}
		}
		switch role {
		case "first_frame":
			first++
		case "last_frame":
			last++
		case "", "reference_image":
			regular++
		default:
			return fmt.Errorf("image role must be first_frame, last_frame or reference_image")
		}
	}
	if first > 1 || last > 1 {
		return fmt.Errorf("only one first_frame and one last_frame are allowed")
	}
	if last > 0 && first == 0 {
		return fmt.Errorf("last_frame requires first_frame")
	}
	if regular > 0 && first+last > 0 {
		return fmt.Errorf("first/last frames cannot be mixed with reference images")
	}
	return nil
}
