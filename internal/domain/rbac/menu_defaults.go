package rbac

// MenuDefaults is the initialization policy input for missing menu metadata.
type MenuDefaults struct {
	TitleEn, TitleJa, Icon, Path, Name, Component, ParentTitle string
	KeepAlive                                                  bool
	SortOrder                                                  uint
}

func (current *Menu) SeedUpdate(seed MenuDefaults, pid *uint) MenuFieldUpdate {
	var update MenuFieldUpdate

	if seed.TitleEn != "" && current.TitleEn() != seed.TitleEn {
		titleEn := seed.TitleEn
		update.TitleEn = &titleEn
	}

	if seed.TitleJa != "" && current.TitleJa() != seed.TitleJa {
		titleJa := seed.TitleJa
		update.TitleJa = &titleJa
	}

	if current.Icon() == "" && seed.Icon != "" {
		icon := seed.Icon
		update.Icon = &icon
	}

	if current.Path() == "" && seed.Path != "" {
		path := seed.Path
		update.Path = &path
	}

	if current.Name() == "" && seed.Name != "" {
		name := seed.Name
		update.Name = &name
	}

	if current.Component() == "" && seed.Component != "" {
		component := seed.Component
		update.Component = &component
	}

	if !current.KeepAlive() && seed.KeepAlive {
		keepAlive := seed.KeepAlive
		update.KeepAlive = &keepAlive
	}

	if current.SortOrder() == 0 && seed.SortOrder != 0 {
		sortOrder := seed.SortOrder
		update.SortOrder = &sortOrder
	}

	if seed.ParentTitle != "" {
		if current.ParentID() == nil || *current.ParentID() == 0 {
			value := *pid
			update.ParentID = &value
		}
	} else if current.ParentID() == nil {
		zero := uint(0)
		update.ParentID = &zero
	}

	return update
}
