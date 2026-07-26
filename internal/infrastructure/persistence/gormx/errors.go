package gormx

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// mysqlDuplicateEntryErrNo MySQL 唯一键冲突错误码。
const mysqlDuplicateEntryErrNo = 1062

// IsDuplicateKeyError 判断是否唯一键冲突。
// 逻辑与原 service/user_service/exp_service.go 的 isDuplicateKeyError 完全一致。
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlDuplicateEntryErrNo
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "duplicate entry") || strings.Contains(msg, "unique constraint failed")
}
