// Package bootstrap 是应用组合根：wire 注入器与 App 生命周期。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"f_ccg_video_back_ddd/internal/infrastructure/cache"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/ipregion"
	"f_ccg_video_back_ddd/internal/infrastructure/logger"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongodb"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"
	"f_ccg_video_back_ddd/internal/infrastructure/search"
	"f_ccg_video_back_ddd/internal/interfaces/http/router"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
)

// Infrastructure 聚合需要显式按序关闭的基础设施客户端。
//
// 关闭顺序由 App.Stop 控制，与原 core.GracefulStop 完全一致：
// Mongo → MySQL → Redis → ES → IPDB。
type Infrastructure struct {
	Mongo *mongodb.Mongo
	MySQL *mysql.DB
	Redis *cache.Redis
	ES    *search.ES
	IPDB  *ipregion.Searcher
}

// NewInfrastructure 组装基础设施句柄集合。
//
// 形参顺序即 wire 生成的构建顺序，刻意与原 core.InitCore 的初始化次序保持一致：
// logger → MySQL → ES → Redis → Mongo → IPDB → Casbin。
// 这样多个中间件同时不可用时，首个失败点与原项目相同。
func NewInfrastructure(
	_ *logger.Logger,
	db *mysql.DB,
	es *search.ES,
	redisClient *cache.Redis,
	mongo *mongodb.Mongo,
	ipdb *ipregion.Searcher,
	_ *casbin.SyncedEnforcer,
) *Infrastructure {
	return &Infrastructure{
		Mongo: mongo,
		MySQL: db,
		Redis: redisClient,
		ES:    es,
		IPDB:  ipdb,
	}
}

// Stop 按原 core.GracefulStop 的顺序关闭基础设施连接（幂等）。
func (i *Infrastructure) Stop() {
	if i == nil {
		return
	}

	i.Mongo.Stop()
	i.MySQL.Stop()
	i.Redis.Stop()
	i.ES.Stop()
	i.IPDB.Stop()
}

// App 是应用运行期聚合体：HTTP 服务 + 后台组件 + 基础设施生命周期。
type App struct {
	cfgProvider *config.Provider
	lg          *logger.Logger
	router      *router.Router
	detached    *ctxutil.DetachedTaskGroup
	runtime     *RuntimeComponents
	infra       *Infrastructure
}

// NewApp 组装应用运行期聚合体。
//
// infra 放在首位：保证 wire 先按原顺序建立全部基础设施连接，
// 再构建仓储 / 应用服务 / HTTP 处理器。
func NewApp(
	infra *Infrastructure,
	cfgProvider *config.Provider,
	lg *logger.Logger,
	httpRouter *router.Router,
	detached *ctxutil.DetachedTaskGroup,
	runtime *RuntimeComponents,
) *App {
	return &App{
		cfgProvider: cfgProvider,
		lg:          lg,
		router:      httpRouter,
		detached:    detached,
		runtime:     runtime,
		infra:       infra,
	}
}

// Start 启动后台组件，语义与原 core.InitCore 尾部的后台启动序列一致。
// 单个组件启动失败只记录日志、不阻断 HTTP 服务启动（与原实现一致）。

func (a *App) Start(ctx context.Context) error {
	a.runtime.Start(ctx)

	return nil
}

func (a *App) log() *zap.Logger {
	if a == nil || a.lg == nil {
		return zap.NewNop()
	}

	return a.lg.Zap()
}

// Serve 启动 HTTP 服务，并在收到 SIGINT/SIGTERM 后执行优雅关闭。
// 监听与关闭错误返回给 main 统一转换退出码，确保后台组件和基础设施仍会显式停机。
func (a *App) Serve() error {
	if a == nil || a.router == nil || a.cfgProvider == nil {
		return errors.New("HTTP server is not initialized")
	}

	rootConfig := a.cfgProvider.Get()
	if rootConfig == nil || rootConfig.Server == nil {
		return errors.New("server config is not initialized")
	}

	engine := a.router.BuildEngine()

	srv := &http.Server{
		Addr:              rootConfig.Server.Addr,
		Handler:           engine,
		ReadHeaderTimeout: ctxutil.HTTPReadHeaderTimeout,
		ReadTimeout:       ctxutil.HTTPReadTimeout,
		WriteTimeout:      ctxutil.HTTPWriteTimeout,
		IdleTimeout:       ctxutil.HTTPIdleTimeout,
		MaxHeaderBytes:    ctxutil.HTTPMaxHeaderBytes,
	}

	errCh := make(chan error, 1)

	go func() {
		a.log().Info("http server starting", zap.String("addr", srv.Addr))

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err

			return
		}

		errCh <- nil
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		if err != nil {
			a.log().Error("http server run error", zap.Error(err))

			return fmt.Errorf("run HTTP server: %w", err)
		}

		return nil
	case sig := <-quit:
		a.log().Info("received shutdown signal", zap.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), ctxutil.HTTPShutdownTimeout)
	defer cancel()

	var shutdownErr error
	if err := srv.Shutdown(ctx); err != nil {
		shutdownErr = fmt.Errorf("shutdown HTTP server: %w", err)
		a.log().Error("http server shutdown error", zap.Error(err))

		if closeErr := srv.Close(); closeErr != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close HTTP server: %w", closeErr))
		}
	}

	serveErr := <-errCh
	if serveErr != nil {
		a.log().Error("http server exit error", zap.Error(serveErr))
		serveErr = fmt.Errorf("exit HTTP server: %w", serveErr)
	}

	a.log().Info("http server stopped")

	return errors.Join(shutdownErr, serveErr)
}

// Stop 在 HTTP 关闭后按序停止后台任务与基础设施。
// 顺序与原 core.GracefulStop 一致：
// 转码消费者 → 视频调度 → River → Cron → detached 任务 → Mongo → MySQL → Redis → ES → IPDB → Logger。
func (a *App) Stop(ctx context.Context) error {
	a.log().Info("graceful stop starting")

	a.runtime.Stop(ctx)

	detachedCtx, cancel := context.WithTimeout(context.Background(), ctxutil.AsyncLongTimeout)
	if err := a.detached.Stop(detachedCtx); err != nil {
		a.log().Warn("stop detached tasks timed out", zap.Error(err))
	}

	cancel()

	a.infra.Stop()
	a.lg.Stop()

	// Logger 已 Sync，后续尽量少打日志；这里用标准库无法再保证刷盘，故不再输出。
	return nil
}
