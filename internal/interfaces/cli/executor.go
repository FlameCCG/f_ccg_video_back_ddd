package cli

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/pkg/constants"

	"go.uber.org/zap"
)

const (
	bannerImagesDir        = "init_resource/images"
	videoPartitionsSQLFile = "init_resource/sql/video_partitions.sql"
)

//go:embed mappings/videos_mapping.json
var videosMapping string

//go:embed mappings/users_mapping.json
var usersMapping string

// Executor 是一次性运维任务的可测试执行入口。
//
// 任务实现只返回 error；退出码由 cmd/server 的组合根统一决定，任何用例都不调用 os.Exit。
type Executor struct {
	run func(context.Context, []string) error
}

// Execute 执行任务。args 是 flag.Parse 后剩余的位置参数。
func (e *Executor) Execute(ctx context.Context, args []string) error {
	if e == nil || e.run == nil {
		return errors.New("CLI task executor is not initialized")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	return e.run(ctx, args)
}

// DatabaseMigrator 是 -db 所需的最小用例端口。
type DatabaseMigrator interface {
	AutoMigrate() error
}

// VideoPartitionSeeder 是 -vp 所需的最小用例端口。
type VideoPartitionSeeder interface {
	SeedVideoPartitions(ctx context.Context, sqlPath string) (int, int, string, error)
}

// BannerStorageResolver 是 -br 对存储解析器的窄化端口。
type BannerStorageResolver interface {
	Resolve() (provisioningapp.StorageDriver, error)
}

// SuperUserProvisioner 是 -su 所需的最小用例端口。
type SuperUserProvisioner interface {
	Provision(ctx context.Context, args []string) (*provisioningapp.SuperUserResult, error)
}

// IndexCreator 是 -es 所需的最小用例端口。
type IndexCreator interface {
	CreateIndexV2(index, mapping string) error
}

// ResourceSynchronizer 是 -rs 所需的最小用例端口。
type ResourceSynchronizer interface {
	SyncApiResources(ctx context.Context) (rbacapp.ResourceSyncResult, error)
}

// RBACInitializer 是 -rbac 所需的最小用例端口。
type RBACInitializer interface {
	InitDefaultRBAC(ctx context.Context) error
}

// NewMigrateDBExecutor 构造 -db 执行器。
func NewMigrateDBExecutor(migrator DatabaseMigrator) *Executor {
	return &Executor{run: func(_ context.Context, _ []string) error {
		// Migrator 保留原「数据库迁移成功/失败」日志，CLI 只负责把错误交给 main 转成退出码 1。
		return migrator.AutoMigrate()
	}}
}

// NewVideoPartitionsExecutor 构造 -vp 执行器。
func NewVideoPartitionsExecutor(seeder VideoPartitionSeeder, logger *zap.Logger) *Executor {
	logger = loggerOrNop(logger)

	return &Executor{run: func(ctx context.Context, _ []string) error {
		imported, normalizedExtras, source, err := seeder.SeedVideoPartitions(
			ctx,
			videoPartitionsSQLFile,
		)
		if err != nil {
			logger.Error("video partitions init failed", zap.Error(err))

			return err
		}

		logger.Info("video partitions init success",
			zap.String("source", source),
			zap.Int("imported", imported),
			zap.Int("normalizedExtras", normalizedExtras))

		return nil
	}}
}

// NewBannerExecutor 构造 -br 执行器。
func NewBannerExecutor(
	resolver BannerStorageResolver,
	repo provisioningapp.BannerRepository,
	logger *zap.Logger,
) *Executor {
	logger = loggerOrNop(logger)

	return &Executor{run: func(ctx context.Context, _ []string) error {
		driver, err := resolver.Resolve()
		if err != nil {
			logger.Error("Failed to resolve storage driver", zap.Error(err))

			return err
		}

		processed, err := provisioningapp.NewBannerService(driver, repo, logger).Provision(ctx)
		if err != nil {
			logBannerError(logger, err)

			return err
		}

		logger.Info("Banner migration completed successfully", zap.Int("total_processed", processed))

		return nil
	}}
}

// NewSuperUserExecutor 构造 -su 执行器。
func NewSuperUserExecutor(service SuperUserProvisioner, logger *zap.Logger) *Executor {
	logger = loggerOrNop(logger)

	return &Executor{run: func(ctx context.Context, args []string) error {
		result, err := service.Provision(ctx, args)
		if err != nil {
			logSuperUserError(logger, err)

			return err
		}

		if result.UserCreated {
			logger.Info("创建超级管理员用户成功",
				zap.String("username", result.Username),
				zap.String("email", result.Username+"@example.com"))
		} else {
			logger.Info("更新超级管理员用户成功（密码/等级/经验已更新）",
				zap.String("username", result.Username))
		}

		logger.Info("绑定管理员角色成功", zap.String("role", result.RoleName))

		return nil
	}}
}

// NewESIndexExecutor 构造 -es 执行器。
func NewESIndexExecutor(indexes IndexCreator, logger *zap.Logger) *Executor {
	logger = loggerOrNop(logger)

	return &Executor{run: func(_ context.Context, _ []string) error {
		if err := indexes.CreateIndexV2(constants.VideoSearchIndex, videosMapping); err != nil {
			err = fmt.Errorf("create video index: %w", err)
			logger.Error("es index task failed", zap.Error(err))

			return err
		}

		if err := indexes.CreateIndexV2(constants.UserSearchIndex, usersMapping); err != nil {
			err = fmt.Errorf("create user index: %w", err)
			logger.Error("es index task failed", zap.Error(err))

			return err
		}

		return nil
	}}
}

// NewResourceSyncExecutor 构造 -rs 执行器。
func NewResourceSyncExecutor(syncer ResourceSynchronizer, logger *zap.Logger) *Executor {
	logger = loggerOrNop(logger)

	return &Executor{run: func(ctx context.Context, _ []string) error {
		result, err := syncer.SyncApiResources(ctx)
		if err != nil {
			logger.Error("resource sync failed", zap.Error(err))

			return err
		}

		logger.Info("resource sync success",
			zap.Int("total", result.Total),
			zap.Int("created", result.Created),
			zap.Int("updated", result.Updated),
			zap.Int("deleted", result.Deleted))

		return nil
	}}
}

// NewRBACInitExecutor 构造 -rbac 执行器。
func NewRBACInitExecutor(initializer RBACInitializer, logger *zap.Logger) *Executor {
	logger = loggerOrNop(logger)

	return &Executor{run: func(ctx context.Context, _ []string) error {
		if err := initializer.InitDefaultRBAC(ctx); err != nil {
			logger.Error("rbac init failed", zap.Error(err))

			return err
		}

		logger.Info("rbac init success")

		return nil
	}}
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

func logBannerError(logger *zap.Logger, err error) {
	switch {
	case errors.Is(err, provisioningapp.ErrBannerImagesDirUnreadable):
		logger.Error("Failed to read images directory",
			zap.String("dir", bannerImagesDir),
			zap.Error(err))
	case strings.HasPrefix(err.Error(), "Failed to migrate banner table"):
		logger.Error("Failed to migrate banner table", zap.Error(err))
	default:
		logger.Error("Banner migration failed", zap.Error(err))
	}
}

func logSuperUserError(logger *zap.Logger, err error) {
	if errors.Is(err, provisioningapp.ErrSuperUserArgsRequired) {
		logger.Error(provisioningapp.ErrSuperUserArgsRequired.Error())

		return
	}

	message := err.Error()
	if separator := strings.Index(message, ": "); separator > 0 {
		logger.Error(message[:separator], zap.Error(errors.New(message[separator+2:])))

		return
	}

	logger.Error(message)
}
