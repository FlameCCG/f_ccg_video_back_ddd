package cli

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestExecutorWithoutTaskReturnsError(t *testing.T) {
	t.Parallel()

	{
		err := (*Executor)(nil).Execute(context.Background(), nil)
		require.Error(t, err, "nil executor should fail")
	}
}

func TestMigrateDBExecutorForwardsResult(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("migrate failed")
	migrator := &fakeMigrator{err: wantErr}

	err := NewMigrateDBExecutor(migrator).Execute(context.Background(), nil)
	require.ErrorIs(t, err, wantErr, "Execute() error = %v, want %v", err, wantErr)

	require.Equal(t, 1, migrator.calls, "AutoMigrate calls = %d, want 1", migrator.calls)
}

func TestVideoPartitionsExecutorLogsOriginalResult(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	seeder := &fakePartitionSeeder{imported: 12, normalized: 3, source: "seed.sql"}

	{
		err := NewVideoPartitionsExecutor(seeder, logger).Execute(context.Background(), nil)
		require.NoError(t, err, "Execute() error = %v", err)
	}

	require.Equal(t, videoPartitionsSQLFile, seeder.path, "seed path = %q, want %q", seeder.path, videoPartitionsSQLFile)

	entries := logs.FilterMessage("video partitions init success").All()
	require.Len(t, entries, 1, "success log count = %d, want 1", len(entries))

	fields := entries[0].ContextMap()
	require.EqualValues(t, "seed.sql", fields["source"], "unexpected log fields: %#v", fields)
	require.EqualValues(t, int64(12), fields["imported"], "unexpected log fields: %#v", fields)
	require.EqualValues(t, int64(3), fields["normalizedExtras"], "unexpected log fields: %#v", fields)
}

func TestBannerExecutorRunsRealProvisioningUseCase(t *testing.T) {
	logger, logs := observedLogger()
	driver := &fakeBannerDriver{}
	resolver := &fakeBannerResolver{driver: driver}
	repo := &fakeBannerRepository{}

	tempDir := t.TempDir()

	imagesDir := filepath.Join(tempDir, "init_resource", "images")
	{
		err := os.MkdirAll(imagesDir, 0o750)
		require.NoError(t, err)
	}

	{
		err := os.WriteFile(filepath.Join(imagesDir, "首页轮播-1.png"), []byte("image"), 0o600)
		require.NoError(t, err)
	}

	t.Chdir(tempDir)

	{
		err := NewBannerExecutor(resolver, repo, logger).Execute(context.Background(), nil)
		require.NoError(t, err, "Execute() error = %v", err)
	}

	require.Equal(t, 1, resolver.calls, "unexpected calls: resolver=%d driver=%d migrate=%d upsert=%d", resolver.calls, driver.calls, repo.migrateCalls, repo.upsertCalls)
	require.Equal(t, 1, driver.calls, "unexpected calls: resolver=%d driver=%d migrate=%d upsert=%d", resolver.calls, driver.calls, repo.migrateCalls, repo.upsertCalls)
	require.Equal(t, 1, repo.migrateCalls, "unexpected calls: resolver=%d driver=%d migrate=%d upsert=%d", resolver.calls, driver.calls, repo.migrateCalls, repo.upsertCalls)
	require.Equal(t, 1, repo.upsertCalls, "unexpected calls: resolver=%d driver=%d migrate=%d upsert=%d", resolver.calls, driver.calls, repo.migrateCalls, repo.upsertCalls)

	require.Equal(t, enum.BannerTypeHome, repo.bannerType, "banner type = %v, want home", repo.bannerType)

	require.Equal(t, 1, logs.FilterMessage("Banner migration completed successfully").Len(), "missing Banner completion log")
}

func TestBannerExecutorLogsResolveFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("storage unavailable")
	logger, logs := observedLogger()
	executor := NewBannerExecutor(&fakeBannerResolver{err: wantErr}, &fakeBannerRepository{}, logger)

	{
		err := executor.Execute(context.Background(), nil)
		require.ErrorIs(t, err, wantErr, "Execute() error = %v, want %v", err, wantErr)
	}

	require.Equal(t, 1, logs.FilterMessage("Failed to resolve storage driver").Len(), "missing original storage resolution error log")
}

func TestSuperUserExecutorForwardsArgsAndLogsResult(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	service := &fakeSuperUserProvisioner{result: &provisioningapp.SuperUserResult{
		Username:    "admin",
		UserID:      7,
		UserCreated: true,
		RoleName:    constants.AdminUserRole,
		RoleCreated: true,
	}}
	args := []string{"admin", "secret"}

	{
		err := NewSuperUserExecutor(service, logger).Execute(context.Background(), args)
		require.NoError(t, err, "Execute() error = %v", err)
	}

	require.Len(t, service.args, 2, "forwarded args = %#v", service.args)
	require.Equal(t, "admin", service.args[0], "forwarded args = %#v", service.args)
	require.Equal(t, "secret", service.args[1], "forwarded args = %#v", service.args)

	require.Equal(t, 1, logs.FilterMessage("创建超级管理员用户成功").Len(), "missing original super-user success logs")
	require.Equal(t, 1, logs.FilterMessage("绑定管理员角色成功").Len(), "missing original super-user success logs")
}

func TestSuperUserExecutorLogsMissingArgs(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	service := &fakeSuperUserProvisioner{err: provisioningapp.ErrSuperUserArgsRequired}

	err := NewSuperUserExecutor(service, logger).Execute(context.Background(), nil)
	require.ErrorIs(t, err, provisioningapp.ErrSuperUserArgsRequired, "Execute() error = %v", err)

	require.Equal(t, 1, logs.FilterMessage(provisioningapp.ErrSuperUserArgsRequired.Error()).Len(), "missing original missing-arguments log")
}

func TestESIndexExecutorUsesEmbeddedMappingsInOrder(t *testing.T) {
	t.Parallel()

	indexer := &fakeIndexCreator{}
	{
		err := NewESIndexExecutor(indexer, zap.NewNop()).Execute(context.Background(), nil)
		require.NoError(t, err, "Execute() error = %v", err)
	}

	require.Len(t, indexer.calls, 2, "CreateIndexV2 calls = %d, want 2", len(indexer.calls))

	require.Equal(t, constants.VideoSearchIndex, indexer.calls[0].index, "index order = %#v", indexer.calls)
	require.Equal(t, constants.UserSearchIndex, indexer.calls[1].index, "index order = %#v", indexer.calls)

	for _, call := range indexer.calls {
		require.Contains(t, call.mapping, "mappings", "mapping for %s was not embedded", call.index)
	}
}

func TestESIndexExecutorStopsOnFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("create failed")
	logger, logs := observedLogger()
	indexer := &fakeIndexCreator{err: wantErr}

	err := NewESIndexExecutor(indexer, logger).Execute(context.Background(), nil)
	require.ErrorIs(t, err, wantErr, "Execute() error = %v, want %v", err, wantErr)

	require.Len(t, indexer.calls, 1, "CreateIndexV2 calls = %d, want 1", len(indexer.calls))

	require.Equal(t, 1, logs.FilterMessage("es index task failed").Len(), "missing original ES task failure log")
}

func TestResourceSyncExecutorLogsCounts(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	syncer := &fakeResourceSynchronizer{result: rbacapp.ResourceSyncResult{
		Total: 10, Created: 2, Updated: 3, Deleted: 1,
	}}

	{
		err := NewResourceSyncExecutor(syncer, logger).Execute(context.Background(), nil)
		require.NoError(t, err, "Execute() error = %v", err)
	}

	entries := logs.FilterMessage("resource sync success").All()
	require.Len(t, entries, 1, "success log count = %d, want 1", len(entries))

	fields := entries[0].ContextMap()
	require.EqualValues(t, int64(10), fields["total"], "unexpected log fields: %#v", fields)
	require.EqualValues(t, int64(2), fields["created"], "unexpected log fields: %#v", fields)
	require.EqualValues(t, int64(3), fields["updated"], "unexpected log fields: %#v", fields)
	require.EqualValues(t, int64(1), fields["deleted"], "unexpected log fields: %#v", fields)
}

func TestRBACInitExecutorReturnsFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("rbac failed")
	logger, logs := observedLogger()
	initializer := &fakeRBACInitializer{err: wantErr}

	err := NewRBACInitExecutor(initializer, logger).Execute(context.Background(), nil)
	require.ErrorIs(t, err, wantErr, "Execute() error = %v, want %v", err, wantErr)

	require.Equal(t, 1, logs.FilterMessage("rbac init failed").Len(), "missing original RBAC failure log")
}

func observedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)

	return zap.New(core), logs
}

type fakeMigrator struct {
	calls int
	err   error
}

func (f *fakeMigrator) AutoMigrate() error {
	f.calls++

	return f.err
}

type fakePartitionSeeder struct {
	imported   int
	normalized int
	source     string
	path       string
	err        error
}

func (f *fakePartitionSeeder) SeedVideoPartitions(
	_ context.Context,
	path string,
) (int, int, string, error) {
	f.path = path

	return f.imported, f.normalized, f.source, f.err
}

type fakeBannerResolver struct {
	driver provisioningapp.StorageDriver
	err    error
	calls  int
}

func (f *fakeBannerResolver) Resolve() (provisioningapp.StorageDriver, error) {
	f.calls++

	return f.driver, f.err
}

type fakeBannerDriver struct {
	calls int
}

func (f *fakeBannerDriver) SaveFile(
	_ context.Context,
	_ string,
	_ int64,
	reader io.Reader,
	_ string,
) (string, error) {
	f.calls++
	_, _ = io.Copy(io.Discard, reader)

	return "/uploads/images/banners/home.png", nil
}

type fakeBannerRepository struct {
	migrateCalls int
	upsertCalls  int
	bannerType   enum.BannerType
}

func (f *fakeBannerRepository) MigrateBannerTable(context.Context) error {
	f.migrateCalls++

	return nil
}

func (f *fakeBannerRepository) UpsertBannerByCover(
	_ context.Context,
	_ string,
	bannerType enum.BannerType,
) (bool, error) {
	f.upsertCalls++
	f.bannerType = bannerType

	return true, nil
}

type fakeSuperUserProvisioner struct {
	args   []string
	result *provisioningapp.SuperUserResult
	err    error
}

func (f *fakeSuperUserProvisioner) Provision(
	_ context.Context,
	args []string,
) (*provisioningapp.SuperUserResult, error) {
	f.args = append([]string(nil), args...)

	return f.result, f.err
}

type indexCall struct {
	index   string
	mapping string
}

type fakeIndexCreator struct {
	calls []indexCall
	err   error
}

func (f *fakeIndexCreator) CreateIndexV2(index, mapping string) error {
	f.calls = append(f.calls, indexCall{index: index, mapping: mapping})

	return f.err
}

type fakeResourceSynchronizer struct {
	result rbacapp.ResourceSyncResult
	err    error
}

func (f *fakeResourceSynchronizer) SyncAPIResources(
	context.Context,
) (rbacapp.ResourceSyncResult, error) {
	return f.result, f.err
}

type fakeRBACInitializer struct {
	err error
}

func (f *fakeRBACInitializer) InitDefaultRBAC(context.Context) error {
	return f.err
}
