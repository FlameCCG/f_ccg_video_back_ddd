package rbac

import (
	"errors"
)

// AlreadyInheritedError 已直接/间接继承目标角色时的业务错误。
func AlreadyInheritedError(targetRole string, kind InheritKind, topParent string) error {
	return errors.New((Inherit{}).AlreadyInheritedMessage(targetRole, kind, topParent))
}

// IndirectUnlinkError 尝试解除间接继承时的业务错误。
func IndirectUnlinkError(targetRole, topParent string) error {
	return errors.New((Inherit{}).IndirectUnlinkMessage(targetRole, topParent))
}
