package config

import (
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
)

func TestMysqlGetDSNEnablesRoundTripOptimizations(t *testing.T) {
	t.Parallel()

	dsn := (Mysql{
		Username: "root",
		Password: "secret",
		Host:     "localhost",
		Port:     3307,
		DBName:   "video",
	}).GetDSN()

	cfg, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}

	if !cfg.InterpolateParams {
		t.Fatal("InterpolateParams = false, want true")
	}

	if cfg.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %s, want 3s", cfg.Timeout)
	}

	if !cfg.ParseTime {
		t.Fatal("ParseTime = false, want true")
	}

	if cfg.Params["charset"] != "utf8mb4" {
		t.Fatalf("charset = %q, want utf8mb4", cfg.Params["charset"])
	}

	if cfg.Loc == nil || cfg.Loc.String() != DefaultMysqlTimezone {
		t.Fatalf("location = %v, want %s", cfg.Loc, DefaultMysqlTimezone)
	}
}

func TestMysqlLoadLocationUsesExplicitTimezone(t *testing.T) {
	t.Parallel()

	mysqlCfg := Mysql{Timezone: "UTC"}

	location, err := mysqlCfg.LoadLocation()
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	if location != time.UTC {
		t.Fatalf("location = %v, want UTC", location)
	}
}
