// Package logger 创建并回收 zap 日志器。
//
// logger.go 组织编码、输出、日志文件轮转与 Sync，provider.go 导出 Wire 构造器。
// 日志实例通过构造器传给使用者；进程停止时在其他资源清理后刷盘，避免最后的错误丢失。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package logger

import (
	"os"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 包装 zap 日志器并负责其生命周期。
type Logger struct {
	lg *zap.Logger
}

// Zap 返回底层 zap 日志器。
func (l *Logger) Zap() *zap.Logger {
	if l == nil || l.lg == nil {
		return zap.NewNop()
	}

	return l.lg
}

// New 依据日志配置构建 zap 日志器。
// 语义与原 core.InitZapLogger 一致。
func New(cfg *config.Logger) *Logger {
	// 与原实现一致：logger 配置段缺失属于配置错误，直接以 nil 解引用 panic 暴露，
	// 不静默降级到默认日志文件（避免日志写到意料之外的位置）。
	writeSyncerDebug := getLogWriter(cfg.DebugFileName, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
	writeSyncerInfo := getLogWriter(cfg.InfoFileName, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
	writeSyncerWarn := getLogWriter(cfg.WarnFileName, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
	writeSyncerError := getLogWriter(cfg.ErrorFileName, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)

	encoder := getEncoder()

	// 注意这里用 levelEnabler 控制每个 Core 只接受对应级别
	debugCore := zapcore.NewCore(encoder, writeSyncerDebug, levelEnabler(zapcore.DebugLevel))
	infoCore := zapcore.NewCore(encoder, writeSyncerInfo, levelEnabler(zapcore.InfoLevel))
	warnCore := zapcore.NewCore(encoder, writeSyncerWarn, levelEnabler(zapcore.WarnLevel))
	errorCore := zapcore.NewCore(encoder, writeSyncerError, levelEnabler(zapcore.ErrorLevel))

	var cores []zapcore.Core

	cores = append(cores, debugCore, infoCore, warnCore, errorCore)

	if cfg.EnableConsole {
		var consoleCore zapcore.Core
		if cfg.Development {
			consoleCore = zapcore.NewCore(
				getConsoleEncoder(),
				zapcore.Lock(os.Stdout),
				zapcore.DebugLevel,
			)
		} else {
			consoleCore = zapcore.NewCore(
				getProductionConsoleEncoder(),
				zapcore.Lock(os.Stdout),
				zapcore.InfoLevel,
			)
		}

		cores = append(cores, consoleCore)
	}

	core := zapcore.NewTee(cores...)
	lg := zap.New(core, zap.AddCaller())

	return &Logger{lg: lg}
}

// Stop 刷盘并释放日志资源（应在其它组件停止后调用）。
func (l *Logger) Stop() {
	if l == nil || l.lg == nil {
		return
	}

	// Sync 在部分平台对 stdout 可能返回 EINVAL，忽略即可。
	_ = l.lg.Sync()
	l.lg = nil
}

// 自定义 LevelEnabler，只允许某一个级别.
func levelEnabler(level zapcore.Level) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(l zapcore.Level) bool {
		return l == level
	})
}

// 构建 JSON 编码器，用于格式化日志输出内容（文件输出）.
func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder         // 时间格式：ISO8601
	encoderConfig.TimeKey = "time"                                // 设置时间字段名
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder       // 日志级别大写
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder // 持续时间格式化为秒
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder       // 记录调用者位置时，显示成哪种格式

	return zapcore.NewJSONEncoder(encoderConfig)
}

// 获取开发环境控制台编码器（带颜色）.
func getConsoleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 彩色级别编码器
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeName = zapcore.FullNameEncoder
	encoderConfig.ConsoleSeparator = " | "

	return zapcore.NewConsoleEncoder(encoderConfig)
}

// 获取生产环境控制台编码器（无颜色，简洁）.
func getProductionConsoleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.ConsoleSeparator = " | "

	return zapcore.NewConsoleEncoder(encoderConfig)
}

// getLogWriter 构建日志输出目标（带滚动策略）.
func getLogWriter(filename string, maxSize, maxBackup, maxAge int) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,  // 输出文件路径
		MaxSize:    maxSize,   // 单文件最大大小（MB）
		MaxBackups: maxBackup, // 最大备份数
		MaxAge:     maxAge,    // 最长保留天数
		Compress:   true,      // 压缩旧日志文件
	}

	return zapcore.AddSync(lumberJackLogger) // 将 lumberjack 封装为 zap 可识别的输出目标
}

// Provide 供 wire 使用的 provider：返回日志器与其刷盘关闭函数，并把日志器
// 显式附加到已装载的配置 Provider，供后续热更新诊断使用。
func Provide(cfg *config.Logger, provider *infraconfig.Provider) (*Logger, func()) {
	lg := New(cfg)
	if provider != nil {
		provider.SetLogger(lg.Zap())
	}
	return lg, lg.Stop
}

// ProvideZap 取出底层 *zap.Logger 供中间件等注入。
func ProvideZap(l *Logger) *zap.Logger {
	return l.Zap()
}
