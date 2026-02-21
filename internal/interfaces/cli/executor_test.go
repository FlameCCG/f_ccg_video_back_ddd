package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestExecutorWithoutTaskReturnsError(t *testing.T) {
	t.Parallel()

	if err := (*Executor)(nil).Execute(context.Background(), nil); err == nil {
		t.Fatal("nil executor should fail")
	}
}

func TestMigrateDBExecutorForwardsResult(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("migrate failed")
	migrator := &fakeMigrator{err: wantErr}

	err := NewMigrateDBExecutor(migrator).Execute(context.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if migrator.calls != 1 {
		t.Fatalf("AutoMigrate calls = %d, want 1", migrator.calls)
	}
}

func TestVideoPartitionsExecutorLogsOriginalResult(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	seeder := &fakePartitionSeeder{imported: 12, normalized: 3, source: "seed.sql"}

	if err := NewVideoPartitionsExecutor(seeder, logger).Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if seeder.path != videoPartitionsSQLFile {
		t.Fatalf("seed path = %q, want %q", seeder.path, videoPartitionsSQLFile)
	}

	entries := logs.FilterMessage("video partitions init success").All()
	if len(entries) != 1 {
		t.Fatalf("success log count = %d, want 1", len(entries))
	}

	fields := entries[0].ContextMap()
	if fields["source"] != "seed.sql" || fields["imported"] != int64(12) ||
		fields["normalizedExtras"] != int64(3) {
		t.Fatalf("unexpected log fields: %#v", fields)
	}
}

func TestBannerExecutorRunsRealProvisioningUseCase(t *testing.T) {
	logger, logs := observedLogger()
	driver := &fakeBannerDriver{}
	resolver := &fakeBannerResolver{driver: driver}
	repo := &fakeBannerRepository{}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	imagesDir := filepath.Join(tempDir, "init_resource", "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "首页轮播-1.png"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	if err := NewBannerExecutor(resolver, repo, logger).Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resolver.calls != 1 || driver.calls != 1 || repo.migrateCalls != 1 || repo.upsertCalls != 1 {
		t.Fatalf("unexpected calls: resolver=%d driver=%d migrate=%d upsert=%d",
			resolver.calls, driver.calls, repo.migrateCalls, repo.upsertCalls)
	}
	if repo.bannerType != enum.BannerTypeHome {
		t.Fatalf("banner type = %v, want home", repo.bannerType)
	}
	if logs.FilterMessage("Banner migration completed successfully").Len() != 1 {
		t.Fatal("missing Banner completion log")
	}
}

func TestBannerExecutorLogsResolveFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("storage unavailable")
	logger, logs := observedLogger()
	executor := NewBannerExecutor(&fakeBannerResolver{err: wantErr}, &fakeBannerRepository{}, logger)

	if err := executor.Execute(context.Background(), nil); !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if logs.FilterMessage("Failed to resolve storage driver").Len() != 1 {
		t.Fatal("missing original storage resolution error log")
	}
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

	if err := NewSuperUserExecutor(service, logger).Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(service.args) != 2 || service.args[0] != "admin" || service.args[1] != "secret" {
		t.Fatalf("forwarded args = %#v", service.args)
	}
	if logs.FilterMessage("创建超级管理员用户成功").Len() != 1 ||
		logs.FilterMessage("绑定管理员角色成功").Len() != 1 {
		t.Fatal("missing original super-user success logs")
	}
}

func TestSuperUserExecutorLogsMissingArgs(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	service := &fakeSuperUserProvisioner{err: provisioningapp.ErrSuperUserArgsRequired}

	err := NewSuperUserExecutor(service, logger).Execute(context.Background(), nil)
	if !errors.Is(err, provisioningapp.ErrSuperUserArgsRequired) {
		t.Fatalf("Execute() error = %v", err)
	}
	if logs.FilterMessage(provisioningapp.ErrSuperUserArgsRequired.Error()).Len() != 1 {
		t.Fatal("missing original missing-arguments log")
	}
}

func TestESIndexExecutorUsesEmbeddedMappingsInOrder(t *testing.T) {
	t.Parallel()

	indexer := &fakeIndexCreator{}
	if err := NewESIndexExecutor(indexer, zap.NewNop()).Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(indexer.calls) != 2 {
		t.Fatalf("CreateIndexV2 calls = %d, want 2", len(indexer.calls))
	}
	if indexer.calls[0].index != constants.VideoSearchIndex ||
		indexer.calls[1].index != constants.UserSearchIndex {
		t.Fatalf("index order = %#v", indexer.calls)
	}
	for _, call := range indexer.calls {
		if !strings.Contains(call.mapping, "mappings") {
			t.Fatalf("mapping for %s was not embedded", call.index)
		}
	}
}

func TestESIndexExecutorStopsOnFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("create failed")
	logger, logs := observedLogger()
	indexer := &fakeIndexCreator{err: wantErr}

	err := NewESIndexExecutor(indexer, logger).Execute(context.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if len(indexer.calls) != 1 {
		t.Fatalf("CreateIndexV2 calls = %d, want 1", len(indexer.calls))
	}
	if logs.FilterMessage("es index task failed").Len() != 1 {
		t.Fatal("missing original ES task failure log")
	}
}

func TestResourceSyncExecutorLogsCounts(t *testing.T) {
	t.Parallel()

	logger, logs := observedLogger()
	syncer := &fakeResourceSynchronizer{result: rbacapp.ResourceSyncResult{
		Total: 10, Created: 2, Updated: 3, Deleted: 1,
	}}

	if err := NewResourceSyncExecutor(syncer, logger).Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	entries := logs.FilterMessage("resource sync success").All()
	if len(entries) != 1 {
		t.Fatalf("success log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["total"] != int64(10) || fields["created"] != int64(2) ||
		fields["updated"] != int64(3) || fields["deleted"] != int64(1) {
		t.Fatalf("unexpected log fields: %#v", fields)
	}
}

func TestRBACInitExecutorReturnsFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("rbac failed")
	logger, logs := observedLogger()
	initializer := &fakeRBACInitializer{err: wantErr}

	err := NewRBACInitExecutor(initializer, logger).Execute(context.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if logs.FilterMessage("rbac init failed").Len() != 1 {
		t.Fatal("missing original RBAC failure log")
	}
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

func (f *fakeResourceSynchronizer) SyncApiResources(
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
