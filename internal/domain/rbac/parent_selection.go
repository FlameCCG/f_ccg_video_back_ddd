package rbac

import "strings"

// ParentSelection 按查询顺序保留去重后的直接与间接父角色。
type ParentSelection struct {
	Direct   []string
	Indirect []string
}

func (Inherit) SelectParents(roleName string, direct, implicit []string) ParentSelection {
	selection := ParentSelection{Direct: make([]string, 0, len(direct)), Indirect: make([]string, 0)}
	directSet := make(map[string]struct{}, len(direct))
	for _, name := range direct {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := directSet[name]; exists {
			continue
		}
		directSet[name] = struct{}{}
		selection.Direct = append(selection.Direct, name)
	}
	seen := make(map[string]struct{})
	for _, name := range implicit {
		name = strings.TrimSpace(name)
		if name == "" || name == roleName {
			continue
		}
		if _, direct := directSet[name]; direct {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		selection.Indirect = append(selection.Indirect, name)
	}
	return selection
}
