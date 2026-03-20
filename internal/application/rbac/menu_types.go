package rbac

// CreateMenuParams 创建菜单用例参数。
type CreateMenuParams struct {
	Title     string
	TitleEn   string
	TitleJa   string
	Icon      string
	Path      string
	Name      string
	Component string
	KeepAlive bool
	ParentID  *uint
	SortOrder uint
	CreatorID uint
}

// UpdateMenuParams 更新菜单用例参数（指针为 nil 表示该字段不更新）。
type UpdateMenuParams struct {
	ID        uint
	Title     *string
	TitleEn   *string
	TitleJa   *string
	Icon      *string
	Path      *string
	Name      *string
	Component *string
	KeepAlive *bool
	ParentID  *uint
	// ParentProvided 区分「没有提交 parentId」与「提交了 parentId」。
	ParentProvided bool
	SortOrder      *uint
}
