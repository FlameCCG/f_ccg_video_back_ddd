package rbac

// RoleIndirectInheritItem 间接继承的角色及其顶层直接父。
type RoleIndirectInheritItem struct {
	// Name 间接继承到的角色名
	Name string `json:"name"`
	// Via 需要解绑的顶层直接父角色名
	Via string `json:"via"`
}

// RoleInheritsResp 角色继承详情：直接 + 间接。
type RoleInheritsResp struct {
	Direct   []string                  `json:"direct"`
	Indirect []RoleIndirectInheritItem `json:"indirect"`
}
