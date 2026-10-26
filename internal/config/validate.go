package config

import (
	"errors"
	"fmt"
	"strings"
)

// minJWTSecretLen JWT 签名密钥最小长度，防止弱密钥与空密钥进入运行时。
const minJWTSecretLen = 32

// ValidateRequired 校验 HTTP 服务启动与热更新必须存在的配置段。
func ValidateRequired(c *Config) error {
	if err := ValidateTranscodeWorker(c); err != nil {
		return err
	}
	return ValidateJWTConfig(c.Jwt)
}

// ValidateMaintenance leaves task-specific connection validation to the
// constructors in each maintenance task's dependency graph.
func ValidateMaintenance(c *Config) error {
	if c == nil {
		return errors.New("config is required")
	}
	return nil
}

// ValidateTranscodeWorker validates the standalone consumer without HTTP JWT settings.
func ValidateTranscodeWorker(c *Config) error {
	if err := ValidateMaintenance(c); err != nil {
		return err
	}
	if c.Transcode == nil {
		return errors.New("transcode config is required")
	}
	return nil
}

// ValidateJWTConfig 统一启动、热更新与管理端写入的 JWT 校验。
func ValidateJWTConfig(j *Jwt) error {
	if j == nil {
		return errors.New("jwt config is required")
	}

	if err := validateJWTSecret("jwt.accessSecret", j.AccessSecret); err != nil {
		return err
	}

	if err := validateJWTSecret("jwt.refreshSecret", j.RefreshSecret); err != nil {
		return err
	}

	return nil
}

func validateJWTSecret(name, secret string) error {
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("%s is required and must not be empty", name)
	}

	if len(secret) < minJWTSecretLen {
		return fmt.Errorf("%s must be at least %d characters", name, minJWTSecretLen)
	}

	return nil
}
