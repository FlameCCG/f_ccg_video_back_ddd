package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Logger 描述各级别日志文件、控制台输出与轮转保留策略。
// 字段只提供构造参数，实际编码与输出行为在 infrastructure/logger 中实现。
type Logger struct {
	Level         string `mapstructure:"level" json:"level"`
	Output        string `mapstructure:"output" json:"output"`
	ConsoleFormat string `mapstructure:"consoleFormat" json:"consoleFormat"`
	Service       string `mapstructure:"service" json:"service"`
	Env           string `mapstructure:"env" json:"env"`
	Instance      string `mapstructure:"instance" json:"instance"`
	DebugFileName string `mapstructure:"debugFileName" json:"debugFileName"`
	InfoFileName  string `mapstructure:"infoFileName" json:"infoFileName"`
	WarnFileName  string `mapstructure:"warnFileName" json:"warnFileName"`
	ErrorFileName string `mapstructure:"errorFileName" json:"errorFileName"`
	EnableConsole bool   `mapstructure:"enableConsole" json:"enableConsole"`
	MaxSize       int    `mapstructure:"maxSize" json:"maxSize"`
	MaxAge        int    `mapstructure:"maxAge" json:"maxAge"`
	MaxBackups    int    `mapstructure:"maxBackups" json:"maxBackups"`
	Development   bool   `mapstructure:"development" json:"development"`
}

// EffectiveOutput preserves the legacy files/enableConsole behavior when output is absent.
func (l *Logger) EffectiveOutput() string {
	if l.Output != "" {
		return l.Output
	}

	if l.EnableConsole {
		return "both"
	}

	return "files"
}

// EffectiveLevel returns the configured threshold or the compatible Info default.
func (l *Logger) EffectiveLevel() string {
	if l.Level == "" {
		return "info"
	}

	return strings.ToLower(l.Level)
}

// EffectiveFormat resolves the console encoding using the deployment mode.
func (l *Logger) EffectiveFormat() string {
	if l.ConsoleFormat != "" {
		return l.ConsoleFormat
	}

	if l.Development {
		return "console"
	}

	return "json"
}

// Validate rejects ambiguous output and rotation settings before a snapshot is published.
func (l *Logger) Validate() error {
	if l == nil {
		return errors.New("logger config is required")
	}

	switch l.EffectiveLevel() {
	case "debug", "info", "warn", "error", "dpanic", "panic", "fatal":
	default:
		return errors.New("invalid logger.level")
	}

	switch l.EffectiveOutput() {
	case "files", "stdout", "both":
	default:
		return errors.New("invalid logger.output")
	}

	switch l.EffectiveFormat() {
	case "json", "console":
	default:
		return errors.New("invalid logger.consoleFormat")
	}

	if l.MaxSize < 0 || l.MaxAge < 0 || l.MaxBackups < 0 {
		return errors.New("logger rotation values must be non-negative")
	}

	if l.EffectiveOutput() == "stdout" {
		return nil
	}

	seen := map[string]bool{}

	for _, name := range []string{l.DebugFileName, l.InfoFileName, l.WarnFileName, l.ErrorFileName} {
		if strings.TrimSpace(name) == "" {
			return errors.New("logger file paths must not be empty")
		}

		abs, err := filepath.Abs(name)
		if err != nil {
			return fmt.Errorf("logger file path: %w", err)
		}

		if seen[abs] {
			return errors.New("logger level files must use different paths")
		}

		seen[abs] = true
	}

	return nil
}

// LoggerRestartRequired ignores the only live setting: the shared atomic log level.
func LoggerRestartRequired(current, next *Logger) bool {
	if current == nil || next == nil {
		return current != next
	}

	a, b := *current, *next
	a.Level, b.Level = "", ""

	return a != b
}
