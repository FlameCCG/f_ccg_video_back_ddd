package bootstrap

import (
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	"f_ccg_video_back_ddd/internal/infrastructure/logger"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/migration"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"

	"github.com/casbin/casbin/v2"
)

// DBTasks 聚合只依赖「配置 + 日志 + MySQL + Casbin」的运维任务。
//
// 对应原 core.InitCore 中 flags.RunDBTasks 的执行点：InitDB 之后、InitES 之前，
// 因此这些任务不要求 ES / Redis / Mongo / IPDB 在线。
// （原 flags.FlagSuperUserInit 在 global.CasbinEnforcer 为 nil 时会自建 enforcer，
// 故 Casbin 属于该任务本就需要的依赖。）
type DBTasks struct {
	// Migrator 承担 -db（AutoMigrate）与 -vp（分区种子导入）。
	Migrator *migration.Migrator
	// SuperUser 承担 -su。
	SuperUser *provisioningapp.SuperUserService

	logger *logger.Logger
	db     *mysql.DB
}

// NewDBTasks 组装数据库类运维任务。
//
// 形参顺序即 wire 生成的构建顺序，与原初始化次序一致：logger → MySQL → Casbin。
func NewDBTasks(
	lg *logger.Logger,
	db *mysql.DB,
	_ *casbin.SyncedEnforcer,
	migrator *migration.Migrator,
	superUser *provisioningapp.SuperUserService,
) *DBTasks {
	return &DBTasks{
		Migrator:  migrator,
		SuperUser: superUser,
		logger:    lg,
		db:        db,
	}
}

// Stop 关闭该任务集合占用的基础设施（幂等）。
func (t *DBTasks) Stop() {
	if t == nil {
		return
	}

	t.db.Stop()
	t.logger.Stop()
}
