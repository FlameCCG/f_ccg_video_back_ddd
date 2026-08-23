package config

import (
	"strings"
	"testing"
)

func TestValidateRequired(t *testing.T) {
	t.Parallel()

	validJWT := &Jwt{
		AccessSecret:  strings.Repeat("a", minJWTSecretLen),
		RefreshSecret: strings.Repeat("b", minJWTSecretLen),
	}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			wantErr: true,
		},
		{
			name:    "missing transcode config",
			config:  &Config{Jwt: validJWT},
			wantErr: true,
		},
		{
			name: "missing jwt config",
			config: &Config{
				Transcode: &Transcode{
					Mp4Enable:  false,
					DashEnable: false,
				},
			},
			wantErr: true,
		},
		{
			name: "empty jwt secrets",
			config: &Config{
				Transcode: &Transcode{},
				Jwt: &Jwt{
					AccessSecret:  "",
					RefreshSecret: "",
				},
			},
			wantErr: true,
		},
		{
			name: "short jwt secrets",
			config: &Config{
				Transcode: &Transcode{},
				Jwt: &Jwt{
					AccessSecret:  "too-short",
					RefreshSecret: strings.Repeat("r", minJWTSecretLen),
				},
			},
			wantErr: true,
		},
		{
			name: "valid minimal config",
			config: &Config{
				Transcode: &Transcode{
					Mp4Enable:  false,
					DashEnable: false,
				},
				Jwt: validJWT,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRequired(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateRequired() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClone 对应原 global.TestCloneConfig。
func TestClone(t *testing.T) {
	t.Parallel()

	src := &Config{Jwt: &Jwt{AccessSecret: "a"}}

	dst := Clone(src)
	if dst == src {
		t.Fatal("Clone should return a new top-level pointer")
	}

	if dst.Jwt != src.Jwt {
		t.Fatal("Clone should shallow-copy nested pointers")
	}

	dst.Jwt = &Jwt{AccessSecret: "b"}
	if src.Jwt.AccessSecret != "a" {
		t.Fatal("replacing field on clone should not mutate source field slot via shared top-level")
	}
}

// TestCloneNil 保证 nil 输入返回空配置而不是 nil（与原 global.CloneConfig 一致）。
func TestCloneNil(t *testing.T) {
	t.Parallel()

	if got := Clone(nil); got == nil {
		t.Fatal("Clone(nil) should return an empty config, not nil")
	}
}
